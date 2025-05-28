// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 THL A29 Limited, a Tencent company.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"encoding/json"
	"fmt"

	"trpc.group/trpc-go/trpc-mcp-go/internal/utils"
)

// parseInitializeResultFromJSON parses a raw JSON message into an InitializeResult
func parseInitializeResultFromJSON(rawMessage *json.RawMessage) (*InitializeResult, error) {
	var result InitializeResult
	if err := json.Unmarshal(*rawMessage, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal InitializeResult: %v", err)
	}
	return &result, nil
}

// parseListPromptsResultFromJSON parses a raw JSON message into a ListPromptsResult
func parseListPromptsResultFromJSON(rawMessage *json.RawMessage) (*ListPromptsResult, error) {
	var result ListPromptsResult
	if err := json.Unmarshal(*rawMessage, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ListPromptsResult: %v", err)
	}
	return &result, nil
}

// parseGetPromptResultFromJSON parses a raw JSON message into a GetPromptResult
func parseGetPromptResultFromJSON(rawMessage *json.RawMessage) (*GetPromptResult, error) {
	var result GetPromptResult
	if err := json.Unmarshal(*rawMessage, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal GetPromptResult: %v", err)
	}
	return &result, nil
}

// parseListResourcesResultFromJSON parses a raw JSON message into a ListResourcesResult
func parseListResourcesResultFromJSON(rawMessage *json.RawMessage) (*ListResourcesResult, error) {
	var result ListResourcesResult
	if err := json.Unmarshal(*rawMessage, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ListResourcesResult: %v", err)
	}
	return &result, nil
}

// parseReadResourceResultFromJSON parses a raw JSON message into a ReadResourceResult
func parseReadResourceResultFromJSON(rawMessage *json.RawMessage) (*ReadResourceResult, error) {
	// Parse JSON object using internal utility function.
	data, err := utils.ParseJSONObject(rawMessage)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	result := &ReadResourceResult{}

	// Parse contents array
	if contentsArray := utils.ExtractArray(data, "contents"); contentsArray != nil {
		for _, item := range contentsArray {
			if contentMap, ok := item.(map[string]interface{}); ok {
				// Parse resource content using internal function.
				uri, mimeType, content, isText := utils.ParseResourceContent(contentMap)

				if isText {
					// Text resource.
					textResource := TextResourceContents{
						URI:      uri,
						MIMEType: mimeType,
						Text:     content,
					}
					result.Contents = append(result.Contents, textResource)
				} else {
					// Binary resource.
					blobResource := BlobResourceContents{
						URI:      uri,
						MIMEType: mimeType,
						Blob:     content,
					}
					result.Contents = append(result.Contents, blobResource)
				}
			}
		}
	}

	return result, nil
}

// parseListToolsResultFromJSON parses a raw JSON message into a ListToolsResult
func parseListToolsResultFromJSON(rawMessage *json.RawMessage) (*ListToolsResult, error) {
	// Parse JSON object using internal utility function.
	data, err := utils.ParseJSONObject(rawMessage)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	result := &ListToolsResult{
		Tools: []Tool{},
	}

	// Parse tools array
	if toolsArray := utils.ExtractArray(data, "tools"); toolsArray != nil {
		for _, item := range toolsArray {
			if toolMap, ok := item.(map[string]interface{}); ok {
				// Parse tool item using internal function.
				name, description, rawSchema, err := utils.ParseToolItem(toolMap)
				if err != nil {
					continue
				}

				tool := Tool{
					Name:           name,
					Description:    description,
					RawInputSchema: rawSchema,
				}

				// Add tool to result
				result.Tools = append(result.Tools, tool)
			}
		}
	}

	// Parse nextCursor if present
	if nextCursor := utils.ExtractString(data, "nextCursor"); nextCursor != "" {
		result.NextCursor = Cursor(nextCursor)
	}

	return result, nil
}
