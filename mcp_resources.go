// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 THL A29 Limited, a Tencent company.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"context"
)

// resourceHandler defines the function type for handling resource reading
type resourceHandler func(ctx context.Context, req *ReadResourceRequest) (ResourceContents, error)

// registeredResource combines a Resource with its handler function
type registeredResource struct {
	Resource *Resource
	Handler  resourceHandler
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
	URITemplate string `json:"uriTemplate"`

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
