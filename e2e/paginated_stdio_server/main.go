// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

// Paginated STDIO server for proxy discovery integration testing.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type listParams struct {
	Cursor string `json:"cursor,omitempty"`
}

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var req request
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			continue
		}
		if req.ID == nil {
			continue
		}

		var result interface{}
		switch req.Method {
		case "initialize":
			result = map[string]interface{}{
				"protocolVersion": "2025-03-26",
				"serverInfo": map[string]string{
					"name":    "paginated-stdio-server",
					"version": "1.0.0",
				},
				"capabilities": map[string]interface{}{
					"tools": map[string]interface{}{},
				},
			}
		case "tools/list":
			result = listTools(req.Params)
		case "tools/call":
			result = map[string]interface{}{
				"content": []map[string]string{{
					"type": "text",
					"text": "ok",
				}},
			}
		case "ping":
			result = map[string]interface{}{}
		default:
			writeJSON(map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"error": map[string]interface{}{
					"code":    -32601,
					"message": "method not found",
				},
			})
			continue
		}

		writeJSON(map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result":  result,
		})
	}
}

func listTools(raw json.RawMessage) map[string]interface{} {
	var params listParams
	_ = json.Unmarshal(raw, &params)

	if params.Cursor == "page-2" {
		return map[string]interface{}{
			"tools": []map[string]interface{}{
				tool("paged-tool-2"),
			},
		}
	}

	return map[string]interface{}{
		"tools": []map[string]interface{}{
			tool("paged-tool-1"),
		},
		"nextCursor": "page-2",
	}
}

func tool(name string) map[string]interface{} {
	return map[string]interface{}{
		"name":        name,
		"description": fmt.Sprintf("%s description", name),
		"inputSchema": map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
	}
}

func writeJSON(v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	fmt.Fprintln(os.Stdout, string(data))
}
