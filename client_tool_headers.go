// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent. All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"net/http"
	"strconv"
	"strings"
)

type toolHeaderField struct {
	path []string
	name string
	kind string
}

type toolHeaderContextKey struct{}

// toolCallHeaders belongs to one CallTool invocation. Definitions from a private
// tools/list response must not be shared across authorization contexts.
type toolCallHeaders struct {
	name   string
	fields []toolHeaderField
}

// toolHeaderFields validates annotations along statically reachable property
// paths. Annotations under refs, arrays or schema alternatives are rejected.
func toolHeaderFields(raw json.RawMessage) ([]toolHeaderField, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var schema interface{}
	if err := json.Unmarshal(raw, &schema); err != nil {
		return nil, err
	}
	var fields []toolHeaderField
	names := make(map[string]bool)
	var visit func(interface{}, []string, bool) error
	visit = func(value interface{}, path []string, reachable bool) error {
		node, ok := value.(map[string]interface{})
		if !ok {
			return nil
		}
		if _, ref := node["$ref"]; ref {
			reachable = false
		}
		if annotation, exists := node["x-mcp-header"]; exists {
			name, valid := annotation.(string)
			kind, _ := node["type"].(string)
			if !valid || !validHeaderToken(name) || !reachable || len(path) == 0 ||
				(kind != "string" && kind != "integer" && kind != "boolean") || names[strings.ToLower(name)] {
				return fmt.Errorf("invalid x-mcp-header annotation at %v", path)
			}
			names[strings.ToLower(name)] = true
			fields = append(fields, toolHeaderField{path: path, name: "Mcp-Param-" + name, kind: kind})
		}
		for key, child := range node {
			switch key {
			case "properties", "patternProperties", "$defs", "definitions", "dependentSchemas", "dependencies":
				children, _ := child.(map[string]interface{})
				for name, next := range children {
					nextPath := append(append([]string(nil), path...), name)
					if err := visit(next, nextPath, reachable && key == "properties"); err != nil {
						return err
					}
				}
			case "items", "additionalItems", "additionalProperties", "unevaluatedItems", "unevaluatedProperties", "contains", "propertyNames", "not", "if", "then", "else", "contentSchema", "allOf", "oneOf", "anyOf", "prefixItems":
				if items, ok := child.([]interface{}); ok {
					for _, next := range items {
						if err := visit(next, path, false); err != nil {
							return err
						}
					}
				} else if err := visit(child, path, false); err != nil {
					return err
				}
			}
		}
		return nil
	}
	err := visit(schema, nil, true)
	return fields, err
}

func validHeaderToken(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || strings.ContainsRune("!#$%&'*+-.^_`|~", c) {
			continue
		}
		return false
	}
	return true
}

func filterToolHeaders(result *ListToolsResult) {
	valid := result.Tools[:0]
	for _, tool := range result.Tools {
		_, err := toolHeaderFields(tool.RawInputSchema)
		if err != nil {
			GetDefaultLogger().Warnf("Ignoring tool %q: %v", tool.Name, err)
			continue
		}
		valid = append(valid, tool)
	}
	result.Tools = valid
}

// toolHeadersForCall discovers a definition for this invocation only. Reusing a
// definition across calls could leak private schemas or miss changed headers.
// Header errors are never fixed by replaying tools/call.
func (c *Client) toolHeadersForCall(ctx context.Context, name string) ([]toolHeaderField, error) {
	request := &ListToolsRequest{}
	seen := make(map[Cursor]bool)
	for {
		result, err := c.ListTools(ctx, request)
		if err != nil {
			return nil, err
		}
		for _, tool := range result.Tools {
			if tool.Name == name {
				return toolHeaderFields(tool.RawInputSchema)
			}
		}
		if result.NextCursor == "" {
			return nil, fmt.Errorf("tool %q has no usable definition", name)
		}
		if seen[result.NextCursor] {
			return nil, fmt.Errorf("repeated tool-list cursor")
		}
		seen[result.NextCursor] = true
		request.Params.Cursor = result.NextCursor
	}
}

func (c *Client) applyToolHeaders(ctx context.Context, header http.Header, req *JSONRPCRequest) error {
	if req.Method != MethodToolsCall {
		return nil
	}
	for name := range header {
		if strings.HasPrefix(strings.ToLower(name), "mcp-param-") {
			header.Del(name)
		}
	}
	params, _ := req.Params.(map[string]interface{})
	name, _ := params["name"].(string)
	definition, ok := ctx.Value(toolHeaderContextKey{}).(toolCallHeaders)
	if !ok || definition.name != name {
		return fmt.Errorf("tool %q has no request-scoped header definition", name)
	}
	for _, field := range definition.fields {
		value := params["arguments"]
		for _, key := range field.path {
			obj, _ := value.(map[string]interface{})
			value = obj[key]
		}
		if value == nil {
			continue
		}
		text, err := toolHeaderValue(value, field.kind)
		if err != nil {
			return fmt.Errorf("tool %q header %s: %w", name, field.name, err)
		}
		header.Set(field.name, encodeMCPHeader(text))
	}
	return nil
}

func toolHeaderValue(value interface{}, kind string) (string, error) {
	switch kind {
	case "string":
		if s, ok := value.(string); ok {
			return s, nil
		}
	case "boolean":
		if b, ok := value.(bool); ok {
			return strconv.FormatBool(b), nil
		}
	case "integer":
		if n, ok := value.(json.Number); ok {
			// Reject extreme exponents before exact arithmetic so an invalid
			// small request cannot allocate an enormous numerator/denominator.
			f, err := n.Float64()
			if err != nil || math.IsNaN(f) || math.IsInf(f, 0) || math.Trunc(f) != f || math.Abs(f) > 1<<53-1 {
				break
			}
			if f == 0 {
				mantissa := strings.Split(strings.ToLower(string(n)), "e")[0]
				if strings.Trim(mantissa, "-+0.") == "" {
					return "0", nil
				}
				break
			}
			r, valid := new(big.Rat).SetString(string(n))
			if valid && r.IsInt() && r.Num().IsInt64() {
				i := r.Num().Int64()
				if i >= -(1<<53-1) && i <= 1<<53-1 {
					return strconv.FormatInt(i, 10), nil
				}
			}
		}
	}
	return "", fmt.Errorf("expected a header-safe %s value", kind)
}
