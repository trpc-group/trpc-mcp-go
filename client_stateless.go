// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent. All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

const (
	protocolVersionMeta    = "io.modelcontextprotocol/protocolVersion"
	clientInfoMeta         = "io.modelcontextprotocol/clientInfo"
	clientCapabilitiesMeta = "io.modelcontextprotocol/clientCapabilities"
	serverInfoMeta         = "io.modelcontextprotocol/serverInfo"
)

func (c *Client) usesStatelessProtocol() bool {
	return c.protocolVersion == ProtocolVersion_2026_07_28
}

// discover adapts the modern discovery response to the existing Initialize
// contract. It does not send initialize or notifications/initialized, establish
// a session, or silently fall back to a different protocol.
func (c *Client) discover(ctx context.Context, initReq *InitializeRequest) (*InitializeResult, error) {
	if _, ok := c.transport.(*streamableHTTPClientTransport); !ok {
		return nil, errors.New("protocol 2026-07-28 requires streamable HTTP in this client")
	}
	if initReq != nil && !isZeroStruct(initReq.Params) {
		return nil, errors.New("stateless discovery uses client options; initialize parameters are not supported")
	}
	raw, err := c.transport.sendRequest(ctx, newJSONRPCRequest(c.requestID.Add(1), "server/discover", nil))
	if err != nil {
		return nil, fmt.Errorf("server discovery failed: %w", err)
	}
	if isErrorResponse(raw) {
		rpcErr, err := parseRawMessageToError(raw)
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("server discovery error: %s (code: %d)", rpcErr.Error.Message, rpcErr.Error.Code)
	}
	var result struct {
		Result
		SupportedVersions []string           `json:"supportedVersions"`
		Capabilities      ServerCapabilities `json:"capabilities"`
		Instructions      string             `json:"instructions"`
	}
	if err := json.Unmarshal(*raw, &result); err != nil {
		return nil, err
	}
	compatible := false
	for _, version := range result.SupportedVersions {
		if version == c.protocolVersion {
			compatible = true
		}
	}
	if !compatible {
		return nil, fmt.Errorf("server does not advertise protocol %s", c.protocolVersion)
	}
	info := Implementation{}
	if serverInfo, ok := result.Meta[serverInfoMeta]; ok {
		data, err := json.Marshal(serverInfo)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(data, &info); err != nil {
			return nil, err
		}
	}
	c.initialized = true
	c.setState(StateInitialized)
	return &InitializeResult{Result: result.Result, ProtocolVersion: c.protocolVersion,
		ServerInfo: info, Capabilities: result.Capabilities, Instructions: result.Instructions}, nil
}

// statelessRequest copies params before adding metadata. UseNumber preserves
// integer arguments, and no caller-owned map is modified during concurrent calls.
func (c *Client) statelessRequest(req *JSONRPCRequest) (*JSONRPCRequest, error) {
	data, err := json.Marshal(req.Params)
	if err != nil {
		return nil, err
	}
	params := make(map[string]interface{})
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&params); err != nil {
		return nil, err
	}
	if params == nil {
		params = make(map[string]interface{})
	}
	meta, ok := params["_meta"].(map[string]interface{})
	if !ok {
		if params["_meta"] != nil {
			return nil, errors.New("request _meta must be an object")
		}
		meta = make(map[string]interface{})
	}
	capabilities := make(map[string]interface{})
	for k, v := range c.capabilities {
		capabilities[k] = v
	}
	// Only tools/call may create a task. Do not claim task-result support for
	// other methods merely because the client supports task continuation.
	if c.tasksEnabled && (req.Method == MethodToolsCall || strings.HasPrefix(req.Method, "tasks/")) {
		capabilities["extensions"] = map[string]interface{}{tasksExtension: map[string]interface{}{}}
	}
	meta[protocolVersionMeta] = c.protocolVersion
	meta[clientInfoMeta] = c.clientInfo
	meta[clientCapabilitiesMeta] = capabilities
	params["_meta"] = meta
	copy := *req
	copy.Params = params
	return &copy, nil
}

func statelessHeaders(header http.Header, req *JSONRPCRequest) {
	header.Del("Mcp-Session-Id")
	header.Del("Last-Event-ID")
	header.Set("MCP-Protocol-Version", ProtocolVersion_2026_07_28)
	header.Set("Mcp-Method", req.Method)
	header.Del("Mcp-Name")
	field := ""
	switch req.Method {
	case MethodToolsCall, MethodPromptsGet:
		field = "name"
	case MethodResourcesRead:
		field = "uri"
	case "tasks/get", "tasks/update", "tasks/cancel":
		field = "taskId"
	}
	params, _ := req.Params.(map[string]interface{})
	if name, ok := params[field].(string); ok {
		header.Set("Mcp-Name", encodeMCPHeader(name))
	}
}

func encodeMCPHeader(value string) string {
	encode := strings.TrimSpace(value) != value || (strings.HasPrefix(value, "=?base64?") && strings.HasSuffix(value, "?="))
	for _, c := range []byte(value) {
		if (c < 0x20 && c != '\t') || c > 0x7e {
			encode = true
			break
		}
	}
	if encode {
		return "=?base64?" + base64.StdEncoding.EncodeToString([]byte(value)) + "?="
	}
	return value
}

// statelessResponse validates the envelope before unwrapping it, including on
// non-2xx HTTP responses. A protocol rejection must not trigger legacy fallback
// or lose its JSON-RPC code/data. Task failures are inside successful results.
func statelessResponse(data []byte, id interface{}) (*json.RawMessage, error) {
	var envelope struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Result  json.RawMessage `json:"result"`
		Error   json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, err
	}
	wantID, err := json.Marshal(id)
	if err != nil {
		return nil, err
	}
	if envelope.JSONRPC != "2.0" || !bytes.Equal(envelope.ID, wantID) {
		return nil, errors.New("invalid stateless response version or request id")
	}
	if len(envelope.Error) != 0 && string(envelope.Error) != "null" {
		if len(envelope.Result) != 0 {
			return nil, errors.New("response contains both result and error")
		}
		raw := json.RawMessage(data)
		return &raw, nil
	}
	var result struct {
		ResultType string `json:"resultType"`
	}
	if err := json.Unmarshal(envelope.Result, &result); err != nil {
		return nil, err
	}
	if result.ResultType != "complete" && result.ResultType != "task" {
		return nil, fmt.Errorf("unsupported stateless resultType %q", result.ResultType)
	}
	return &envelope.Result, nil
}

func validateStatelessResult(raw *json.RawMessage, method string, tasks bool) (*json.RawMessage, error) {
	var result struct {
		ResultType string `json:"resultType"`
	}
	if err := json.Unmarshal(*raw, &result); err != nil {
		return nil, err
	}
	if result.ResultType == "task" && (method != MethodToolsCall || !tasks) {
		return nil, errors.New("unexpected task result for a request without task support")
	}
	return raw, nil
}
