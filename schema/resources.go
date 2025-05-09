package schema

import (
	"encoding/base64"
	"fmt"
)

// Resource represents a known resource that the server can read
type Resource struct {
	// Resource name
	Name string `json:"name"`

	// Resource URI
	URI string `json:"uri"`

	// Resource description (optional)
	Description string `json:"description,omitempty"`

	// MIME type (optional)
	MimeType string `json:"mimeType,omitempty"`

	// Resource size in bytes (optional)
	Size int64 `json:"size,omitempty"`

	// Annotations (optional)
	Annotations *Annotations `json:"annotations,omitempty"`
}

// ResourceTemplate represents a resource template description
type ResourceTemplate struct {
	// Template name
	Name string `json:"name"`

	// URI template
	URITemplate string `json:"uriTemplate"`

	// Resource description (optional)
	Description string `json:"description,omitempty"`

	// MIME type (optional)
	MimeType string `json:"mimeType,omitempty"`

	// Annotations (optional)
	Annotations *Annotations `json:"annotations,omitempty"`
}

// ResourceContents represents resource contents
type ResourceContents struct {
	// Resource URI
	URI string `json:"uri"`

	// MIME type (optional)
	MimeType string `json:"mimeType,omitempty"`
}

// TextResourceContents represents text resource contents
type TextResourceContents struct {
	ResourceContents
	// Text content
	Text string `json:"text"`
}

// BlobResourceContents represents binary resource contents
type BlobResourceContents struct {
	ResourceContents
	// Binary data (base64 encoded)
	Blob []byte `json:"blob"`
}

// ListResourcesResponse represents the response for listing resources
type ListResourcesResponse struct {
	// Resource list
	Resources []Resource `json:"resources"`

	// Next page cursor (optional)
	NextCursor string `json:"nextCursor,omitempty"`
}

// ReadResourceResponse represents the response for reading a resource
type ReadResourceResponse struct {
	// Resource content list
	Contents []interface{} `json:"contents"`
}

// ParseListResourcesResult parses a list resources response
func ParseListResourcesResult(result interface{}) (*ListResourcesResponse, error) {
	// Type assertion to map
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid resource list response format")
	}

	// Create result object
	resourcesResponse := &ListResourcesResponse{}

	// Parse next page cursor
	if cursor, ok := resultMap["nextCursor"].(string); ok {
		resourcesResponse.NextCursor = cursor
	}

	// Parse resource list
	resourcesArray, ok := resultMap["resources"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("missing resource list in response")
	}

	resources := make([]Resource, 0, len(resourcesArray))
	for _, item := range resourcesArray {
		resource, err := parseResourceItem(item)
		if err != nil {
			continue // or return error
		}
		resources = append(resources, resource)
	}
	resourcesResponse.Resources = resources

	return resourcesResponse, nil
}

// parseResourceItem parses a single resource item
func parseResourceItem(item interface{}) (Resource, error) {
	resourceMap, ok := item.(map[string]interface{})
	if !ok {
		return Resource{}, fmt.Errorf("invalid resource format")
	}

	// Create resource object
	resource := Resource{}

	// Extract name and URI (required fields)
	if name, ok := resourceMap["name"].(string); ok {
		resource.Name = name
	} else {
		return Resource{}, fmt.Errorf("resource missing name")
	}

	if uri, ok := resourceMap["uri"].(string); ok {
		resource.URI = uri
	} else {
		return Resource{}, fmt.Errorf("resource missing URI")
	}

	// Extract optional fields
	if description, ok := resourceMap["description"].(string); ok {
		resource.Description = description
	}

	if mimeType, ok := resourceMap["mimeType"].(string); ok {
		resource.MimeType = mimeType
	}

	if size, ok := resourceMap["size"].(float64); ok {
		resource.Size = int64(size)
	}

	// Extract annotations (if present)
	if annotationsMap, ok := resourceMap["annotations"].(map[string]interface{}); ok {
		annotations := &Annotations{}

		// Extract priority
		if priority, ok := annotationsMap["priority"].(float64); ok {
			annotations.Priority = priority
		}

		// Extract audience
		if audienceArray, ok := annotationsMap["audience"].([]interface{}); ok && len(audienceArray) > 0 {
			audience := make([]Role, 0, len(audienceArray))
			for _, a := range audienceArray {
				if roleStr, ok := a.(string); ok {
					audience = append(audience, Role(roleStr))
				}
			}
			annotations.Audience = audience
		}

		resource.Annotations = annotations
	}

	return resource, nil
}

// ParseReadResourceResult parses a read resource response
func ParseReadResourceResult(result interface{}) (*ReadResourceResponse, error) {
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid read resource response format")
	}

	// Create result object
	resourceResponse := &ReadResourceResponse{}

	// Parse content list
	contentsArray, ok := resultMap["contents"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("missing content list in response")
	}

	// Process content items
	contents := make([]interface{}, 0, len(contentsArray))
	for _, item := range contentsArray {
		contentMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		// Check if it's text or binary content
		if text, hasText := contentMap["text"].(string); hasText {
			// This is text resource
			textContent := TextResourceContents{}

			// Set URI (required)
			if uri, ok := contentMap["uri"].(string); ok {
				textContent.URI = uri
			} else {
				continue // URI is required
			}

			// Set MimeType (optional)
			if mimeType, ok := contentMap["mimeType"].(string); ok {
				textContent.MimeType = mimeType
			}

			textContent.Text = text
			contents = append(contents, textContent)

		} else if blobStr, hasBlob := contentMap["blob"].(string); hasBlob {
			// This is binary resource
			blobContent := BlobResourceContents{}

			// Set URI (required)
			if uri, ok := contentMap["uri"].(string); ok {
				blobContent.URI = uri
			} else {
				continue // URI is required
			}

			// Set MimeType (optional)
			if mimeType, ok := contentMap["mimeType"].(string); ok {
				blobContent.MimeType = mimeType
			}

			// Decode base64 data
			blobData, err := base64.StdEncoding.DecodeString(blobStr)
			if err != nil {
				continue // Cannot decode, skip this item
			}

			blobContent.Blob = blobData
			contents = append(contents, blobContent)
		}
	}

	resourceResponse.Contents = contents
	return resourceResponse, nil
}
