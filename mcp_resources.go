// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"context"
	"encoding/json"

	"github.com/yosida95/uritemplate/v3"
)

// resourceHandler defines the function type for handling resource reading
type resourceHandler func(ctx context.Context, req *ReadResourceRequest) (ResourceContents, error)

// resourcesHandler defines the function type for handling resource reading with multiple contents.
type resourcesHandler func(ctx context.Context, req *ReadResourceRequest) ([]ResourceContents, error)

// resourceTemplateHandler defines the function type for handling resource template reading.
type resourceTemplateHandler func(ctx context.Context, req *ReadResourceRequest) ([]ResourceContents, error)

// registeredResource combines a Resource with its handler function
type registeredResource struct {
	Resource *Resource
	Handler  resourcesHandler
}

// registerResourceTemplate combines a ResourceTemplate with its handler function.
type registerResourceTemplate struct {
	resourceTemplate *ResourceTemplate
	Handler          resourceTemplateHandler
}

// Resource represents a known resource that the server can read.
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
	Annotated
}

// ResourceContents represents resource contents
type ResourceContents interface {
	isResourceContents()
}

// TextResourceContents represents text resource contents
type TextResourceContents struct {
	URI      string `json:"uri"`
	MIMEType string `json:"mimeType,omitempty"`
	Text     string `json:"text"`
}

func (t TextResourceContents) isResourceContents() {}

// BlobResourceContents represents binary resource contents
type BlobResourceContents struct {
	URI      string `json:"uri"`
	MIMEType string `json:"mimeType,omitempty"`
	Blob     string `json:"blob"`
}

func (b BlobResourceContents) isResourceContents() {}

// ListResourcesRequest describes a request to list resources.
type ListResourcesRequest struct {
	PaginatedRequest
}

// ListResourcesResult describes a result of listing resources.
type ListResourcesResult struct {
	PaginatedResult
	Resources []Resource `json:"resources"`
}

// ReadResourceRequest describes a request to read a resource.
type ReadResourceRequest struct {
	Request
	Params struct {
		URI       string                 `json:"uri"`
		Arguments map[string]interface{} `json:"arguments,omitempty"`
	} `json:"params"`
}

// ReadResourceResult describes a result of reading a resource.
type ReadResourceResult struct {
	Result
	Contents []ResourceContents `json:"contents"`
}

// ResourceUpdatedNotification represents a notification that a resource has been updated.
type ResourceUpdatedNotification struct {
	Notification
	Params struct {
		URI string `json:"uri"`
	} `json:"params"`
}

// SubscribeRequest describes a request to subscribe to resource updates.
type SubscribeRequest struct {
	Request
	Params struct {
		URI string `json:"uri"`
	} `json:"params"`
}

// UnsubscribeRequest describes a request to unsubscribe from resource updates.
type UnsubscribeRequest struct {
	Request
	Params struct {
		URI string `json:"uri"`
	} `json:"params"`
}

// ResourceTemplate describes a resource template
type ResourceTemplate struct {
	// Template name
	Name string `json:"name"`

	// URI template
	URITemplate *URITemplate `json:"uriTemplate"`

	// Resource description (optional)
	Description string `json:"description,omitempty"`

	// MIME type (optional)
	MimeType string `json:"mimeType,omitempty"`

	// Embed Annotated struct
	Annotated
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

// ResourceListChangedNotification describes a resource list changed notification.
type ResourceListChangedNotification struct {
	Notification
}

// ResourceTemplateOption is a function that configures a ResourceTemplate.
type ResourceTemplateOption func(*ResourceTemplate)

// NewResourceTemplate creates a new ResourceTemplate.
func NewResourceTemplate(uriTemplate string, name string, opts ...ResourceTemplateOption) *ResourceTemplate {
	template := ResourceTemplate{
		URITemplate: &URITemplate{Template: uritemplate.MustNew(uriTemplate)},
		Name:        name,
	}

	for _, opt := range opts {
		opt(&template)
	}

	return &template
}

// WithTemplateDescription sets the description for the ResourceTemplate.
func WithTemplateDescription(description string) ResourceTemplateOption {
	return func(t *ResourceTemplate) {
		t.Description = description
	}
}

// WithTemplateMIMEType sets the MIME type for the ResourceTemplate.
func WithTemplateMIMEType(mimeType string) ResourceTemplateOption {
	return func(t *ResourceTemplate) {
		t.MimeType = mimeType
	}
}

// WithTemplateAnnotations sets the annotations for the ResourceTemplate.
func WithTemplateAnnotations(audience []Role, priority float64) ResourceTemplateOption {
	return func(t *ResourceTemplate) {
		if t.Annotations == nil {
			t.Annotations = &struct {
				Audience []Role  `json:"audience,omitempty"`
				Priority float64 `json:"priority,omitempty"`
			}{}
		}
		t.Annotations.Audience = audience
		t.Annotations.Priority = priority
	}
}

// URITemplate represents a URI template.
type URITemplate struct {
	*uritemplate.Template
}

// MarshalJSON implements the json.Marshaler interface.
func (t *URITemplate) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.Template.Raw())
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (t *URITemplate) UnmarshalJSON(data []byte) error {
	var raw string
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	template, err := uritemplate.New(raw)
	if err != nil {
		return err
	}
	t.Template = template
	return nil
}
