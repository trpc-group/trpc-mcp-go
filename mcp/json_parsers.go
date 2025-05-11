package mcp

import (
	"encoding/json"
	"fmt"
)

// ParseInitializeResultFromJSON parses a raw JSON message into an InitializeResult
func ParseInitializeResultFromJSON(rawMessage *json.RawMessage) (*InitializeResult, error) {
	var result InitializeResult
	if err := json.Unmarshal(*rawMessage, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal InitializeResult: %v", err)
	}
	return &result, nil
}

// ParseListPromptsResultFromJSON parses a raw JSON message into a ListPromptsResult
func ParseListPromptsResultFromJSON(rawMessage *json.RawMessage) (*ListPromptsResult, error) {
	var result ListPromptsResult
	if err := json.Unmarshal(*rawMessage, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ListPromptsResult: %v", err)
	}
	return &result, nil
}

// ParseGetPromptResultFromJSON parses a raw JSON message into a GetPromptResult
func ParseGetPromptResultFromJSON(rawMessage *json.RawMessage) (*GetPromptResult, error) {
	var result GetPromptResult
	if err := json.Unmarshal(*rawMessage, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal GetPromptResult: %v", err)
	}
	return &result, nil
}

// ParseListResourcesResultFromJSON parses a raw JSON message into a ListResourcesResult
func ParseListResourcesResultFromJSON(rawMessage *json.RawMessage) (*ListResourcesResult, error) {
	var result ListResourcesResult
	if err := json.Unmarshal(*rawMessage, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ListResourcesResult: %v", err)
	}
	return &result, nil
}

// ParseReadResourceResultFromJSON parses a raw JSON message into a ReadResourceResult
func ParseReadResourceResultFromJSON(rawMessage *json.RawMessage) (*ReadResourceResult, error) {
	var data map[string]interface{}
	if err := json.Unmarshal(*rawMessage, &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	result := &ReadResourceResult{}

	// Parse contents array
	if contentsData, ok := data["contents"]; ok {
		if contentsArray, ok := contentsData.([]interface{}); ok {
			for _, item := range contentsArray {
				if contentMap, ok := item.(map[string]interface{}); ok {
					// Check if it's text or blob resource
					if _, hasText := contentMap["text"]; hasText {
						// Text resource
						textResource := TextResourceContents{}

						if text, ok := contentMap["text"].(string); ok {
							textResource.Text = text
						}

						if uri, ok := contentMap["uri"].(string); ok {
							textResource.URI = uri
						}

						if mimeType, ok := contentMap["mimeType"].(string); ok {
							textResource.MIMEType = mimeType
						}

						result.Contents = append(result.Contents, textResource)
					} else if _, hasBlob := contentMap["blob"]; hasBlob {
						// Blob resource
						blobResource := BlobResourceContents{}

						if blob, ok := contentMap["blob"].(string); ok {
							blobResource.Blob = blob
						}

						if uri, ok := contentMap["uri"].(string); ok {
							blobResource.URI = uri
						}

						if mimeType, ok := contentMap["mimeType"].(string); ok {
							blobResource.MIMEType = mimeType
						}

						result.Contents = append(result.Contents, blobResource)
					}
				}
			}
		}
	}

	return result, nil
}

// ParseListToolsResultFromJSON parses a raw JSON message into a ListToolsResult
func ParseListToolsResultFromJSON(rawMessage *json.RawMessage) (*ListToolsResult, error) {
	var data map[string]interface{}
	if err := json.Unmarshal(*rawMessage, &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	result := &ListToolsResult{
		Tools: []Tool{},
	}

	// Parse tools array
	if toolsData, ok := data["tools"]; ok {
		if toolsArray, ok := toolsData.([]interface{}); ok {
			for _, item := range toolsArray {
				if toolMap, ok := item.(map[string]interface{}); ok {
					tool := Tool{}

					// Parse tool properties
					if name, ok := toolMap["name"].(string); ok {
						tool.Name = name
					} else {
						continue // Name is required
					}

					if desc, ok := toolMap["description"].(string); ok {
						tool.Description = desc
					}

					// Parse input schema if present
					if schema, ok := toolMap["inputSchema"].(map[string]interface{}); ok {
						// Convert map to JSON then parse to Schema
						schemaBytes, err := json.Marshal(schema)
						if err != nil {
							continue
						}

						// Store as raw JSON for now
						tool.RawInputSchema = schemaBytes
					}

					// Add tool to result
					result.Tools = append(result.Tools, tool)
				}
			}
		}
	}

	// Parse nextCursor if present
	if nextCursor, ok := data["nextCursor"].(string); ok {
		result.NextCursor = Cursor(nextCursor)
	}

	return result, nil
}
