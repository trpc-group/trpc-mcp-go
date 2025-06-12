// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 THL A29 Limited, a Tencent company.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"encoding/json"
	"fmt"

	"github.com/getkin/kin-openapi/openapi3"
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

				// Convert RawInputSchema to InputSchema object.
				if rawSchema != nil {
					// First try to parse rawSchema directly to openapi3.Schema.
					var schema openapi3.Schema
					err := json.Unmarshal(rawSchema, &schema)
					if err == nil {
						tool.InputSchema = &schema
					} else {
						// If the direct parsing fails, it may be due to type mismatch.
						// Use a more flexible processing method.
						var rawSchemaMap map[string]interface{}
						if jsonErr := json.Unmarshal(rawSchema, &rawSchemaMap); jsonErr != nil {
							// If the map cannot be parsed, skip processing.
							continue
						}

						// Process special field types.
						handleSchemaNumberBoolFields(rawSchemaMap)

						// Re-serialize and deserialize.
						fixedData, jsonErr := json.Marshal(rawSchemaMap)
						if jsonErr != nil {
							continue
						}

						var fixedSchema openapi3.Schema
						if jsonErr := json.Unmarshal(fixedData, &fixedSchema); jsonErr != nil {
							continue
						}

						tool.InputSchema = &fixedSchema
					}
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

// processExclusiveField exclusiveMaximum/exclusiveMinimum field, convert number type to boolean type.
func processExclusiveField(schema map[string]interface{}, field string) {
	if value, exists := schema[field]; exists {
		// If it is already a boolean type, no need to process.
		if _, isBool := value.(bool); isBool {
			return
		}

		// If it is a number type, convert to true.
		if _, isNumber := value.(float64); isNumber {
			schema[field] = true
		}
	}
}

// handleSchemaNumberBoolFields Recursively process special field types in schema.
func handleSchemaNumberBoolFields(schema map[string]interface{}) {
	// Process exclusiveMaximum and exclusiveMinimum fields in the current schema.
	processExclusiveField(schema, "exclusiveMaximum")
	processExclusiveField(schema, "exclusiveMinimum")

	// Process required field, if it is a boolean value.
	if value, exists := schema["required"]; exists {
		if boolVal, isBool := value.(bool); isBool {
			// If it is true, convert to empty string array, if it is false, delete the field.
			if boolVal {
				schema["required"] = []string{}
			} else {
				delete(schema, "required")
			}
		}
	}

	// Recursively process properties.
	if props, ok := schema["properties"].(map[string]interface{}); ok {
		for _, propSchema := range props {
			if propMap, isMap := propSchema.(map[string]interface{}); isMap {
				handleSchemaNumberBoolFields(propMap)
			}
		}
	}

	// Recursively process array items.
	if items, ok := schema["items"].(map[string]interface{}); ok {
		handleSchemaNumberBoolFields(items)
	}

	// Recursively process additionalProperties.
	if addProps, ok := schema["additionalProperties"].(map[string]interface{}); ok {
		handleSchemaNumberBoolFields(addProps)
	}

	// Process allOf, oneOf, anyOf arrays.
	processSchemaArray(schema, "allOf")
	processSchemaArray(schema, "oneOf")
	processSchemaArray(schema, "anyOf")
}

// processSchemaArray Process schema array fields.
func processSchemaArray(schema map[string]interface{}, field string) {
	if arr, ok := schema[field].([]interface{}); ok {
		for _, item := range arr {
			if itemMap, isMap := item.(map[string]interface{}); isMap {
				handleSchemaNumberBoolFields(itemMap)
			}
		}
	}
}
