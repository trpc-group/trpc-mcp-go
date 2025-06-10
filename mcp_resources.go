// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 THL A29 Limited, a Tencent company.  All rights reserved.
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
type resourceTemplateHandler func(ctx context.Context, req *ReadResourceRequest) ([]ResourceContents, error)

// registeredResource combines a Resource with its handler function
type registeredResource struct {
	Resource *Resource
	Handler  resourceHandler
}

type registerResourceTemplate struct {
	resourceTemplate *ResourceTemplate
	Handler          resourceTemplateHandler
}

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

type ReadResourceResult struct {
	Result
	Contents []ResourceContents `json:"contents"`
}

type ResourceUpdatedNotification struct {
	Notification
	Params struct {
		URI string `json:"uri"`
	} `json:"params"`
}

type SubscribeRequest struct {
	Request
	Params struct {
		URI string `json:"uri"`
	} `json:"params"`
}

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

type ResourceTemplateOption func(*ResourceTemplate)

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

// WithTemplateDescription adds a description to the ResourceTemplate.
// The description should provide a clear, human-readable explanation of what resources this template represents.
func WithTemplateDescription(description string) ResourceTemplateOption {
	return func(t *ResourceTemplate) {
		t.Description = description
	}
}

// WithTemplateMIMEType sets the MIME type for the ResourceTemplate.
// This should only be set if all resources matching this template will have the same type.
func WithTemplateMIMEType(mimeType string) ResourceTemplateOption {
	return func(t *ResourceTemplate) {
		t.MimeType = mimeType
	}
}

// WithTemplateAnnotations adds annotations to the ResourceTemplate.
// Annotations can provide additional metadata about the template's intended use.
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

type URITemplate struct {
	*uritemplate.Template
}

func (t *URITemplate) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.Template.Raw())
}

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
