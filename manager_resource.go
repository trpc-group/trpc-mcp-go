// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import (
	"context"
	"fmt"
	"sync"
	"time"

	"trpc.group/trpc-go/trpc-mcp-go/internal/errors"
)

// resourceManager manages resources
//
// Resource functionality follows these enabling mechanisms:
//  1. By default, resource functionality is disabled
//  2. When the first resource is registered, resource functionality is automatically enabled without
//     additional configuration
//  3. When resource functionality is enabled but no resources exist, ListResources will return an empty
//     resource list rather than an error
//  4. Clients can determine if the server supports resource functionality through the capabilities
//     field in the initialization response
//
// This design simplifies API usage, eliminating the need for explicit configuration parameters to
// enable or disable resource functionality.
type resourceManager struct {
	// Resource mapping table
	resources map[string]*registeredResource

	// Resource template mapping table
	templates map[string]*registerResourceTemplate

	// Mutex
	mu sync.RWMutex

	// Subscriber mapping table
	subscribers map[string][]chan *JSONRPCNotification

	// Subscriber mutex
	subMu sync.RWMutex

	// Order of resources
	resourcesOrder []string
}

// newResourceManager creates a new resource manager
//
// Note: Simply creating a resource manager does not enable resource functionality,
// it is only enabled when the first resource is added.
func newResourceManager() *resourceManager {
	return &resourceManager{
		resources:   make(map[string]*registeredResource),
		templates:   make(map[string]*registerResourceTemplate),
		subscribers: make(map[string][]chan *JSONRPCNotification),
	}
}

// registerResource registers a resource
func (m *resourceManager) registerResource(resource *Resource, handler resourceHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if resource == nil || resource.URI == "" {
		return
	}

	if _, exists := m.resources[resource.URI]; !exists {
		m.resourcesOrder = append(m.resourcesOrder, resource.URI)
	}

	m.resources[resource.URI] = &registeredResource{
		Resource: resource,
		Handler: func(ctx context.Context, req *ReadResourceRequest) ([]ResourceContents, error) {
			content, err := handler(ctx, req)
			if err != nil {
				return nil, err
			}
			return []ResourceContents{content}, nil
		},
	}
}

// registerResources registers a resource with multiple contents handler.
func (m *resourceManager) registerResources(resource *Resource, handler resourcesHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if resource == nil || resource.URI == "" {
		return
	}

	if _, exists := m.resources[resource.URI]; !exists {
		m.resourcesOrder = append(m.resourcesOrder, resource.URI)
	}

	m.resources[resource.URI] = &registeredResource{
		Resource: resource,
		Handler:  handler,
	}
}

// registerTemplate registers a resource template
func (m *resourceManager) registerTemplate(template *ResourceTemplate, handler resourceTemplateHandler) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if template == nil {
		return fmt.Errorf("template cannot be nil")
	}

	if template.Name == "" {
		return fmt.Errorf("template name cannot be empty")
	}

	if template.URITemplate == nil {
		return fmt.Errorf("template URI cannot be empty")
	}

	if _, exists := m.templates[template.Name]; exists {
		return fmt.Errorf("template %s already exists", template.Name)
	}

	m.templates[template.Name] = &registerResourceTemplate{
		resourceTemplate: template,
		Handler:          handler,
	}

	return nil
}

// getResource retrieves a resource
func (m *resourceManager) getResource(uri string) (*Resource, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if registeredResource, exists := m.resources[uri]; exists {
		return registeredResource.Resource, true
	}
	return nil, false
}

// getResources retrieves all resources
func (m *resourceManager) getResources() []*Resource {
	m.mu.RLock()
	defer m.mu.RUnlock()

	orderedResources := make([]*Resource, 0, len(m.resources))
	for _, uri := range m.resourcesOrder {
		if registeredResource, exists := m.resources[uri]; exists {
			orderedResources = append(orderedResources, registeredResource.Resource)
		}
	}

	return orderedResources
}

// getTemplates retrieves all resource templates
func (m *resourceManager) getTemplates() []*ResourceTemplate {
	m.mu.RLock()
	defer m.mu.RUnlock()

	templates := make([]*ResourceTemplate, 0, len(m.templates))
	for _, template := range m.templates {
		templates = append(templates, template.resourceTemplate)
	}
	return templates
}

// subscribe subscribes to resource updates
func (m *resourceManager) subscribe(uri string) chan *JSONRPCNotification {
	m.subMu.Lock()
	defer m.subMu.Unlock()

	ch := make(chan *JSONRPCNotification, 10)
	m.subscribers[uri] = append(m.subscribers[uri], ch)
	return ch
}

// unsubscribe cancels a subscription
func (m *resourceManager) unsubscribe(uri string, ch chan *JSONRPCNotification) {
	m.subMu.Lock()
	defer m.subMu.Unlock()

	subs := m.subscribers[uri]
	for i, sub := range subs {
		if sub == ch {
			close(ch)
			subs = append(subs[:i], subs[i+1:]...)
			break
		}
	}
	if len(subs) == 0 {
		delete(m.subscribers, uri)
	} else {
		m.subscribers[uri] = subs
	}
}

// notifyUpdate notifies about resource updates
func (m *resourceManager) notifyUpdate(uri string) {
	m.subMu.RLock()
	subs := m.subscribers[uri]
	m.subMu.RUnlock()

	// Create jsonrpcNotification params with correct struct type
	notification := Notification{
		Method: "notifications/resources/updated",
		Params: NotificationParams{
			AdditionalFields: map[string]interface{}{
				"uri": uri,
			},
		},
	}

	jsonrpcNotification := newJSONRPCNotification(notification)

	for _, ch := range subs {
		select {
		case ch <- jsonrpcNotification:
		default:
			// Skip this subscriber if the channel is full
		}
	}
}

// handleListResources handles listing resources requests
func (m *resourceManager) handleListResources(ctx context.Context, req *JSONRPCRequest) (JSONRPCMessage, error) {
	resources := m.getResources()

	// Convert []*mcp.Resource to []mcp.Resource for the result
	resultResources := make([]Resource, len(resources))
	for i, resource := range resources {
		resultResources[i] = *resource
	}

	// Create result
	result := ListResourcesResult{
		Resources: resultResources,
	}

	// Return response
	return result, nil
}

// handleReadResource handles reading resource requests
func (m *resourceManager) handleReadResource(ctx context.Context, req *JSONRPCRequest) (JSONRPCMessage, error) {
	// Convert params to map for easier access
	paramsMap, ok := req.Params.(map[string]interface{})
	if !ok {
		return newJSONRPCErrorResponse(req.ID, ErrCodeInvalidParams, errors.ErrInvalidParams.Error(), nil), nil
	}

	// Get resource URI from parameters
	uri, ok := paramsMap["uri"].(string)
	if !ok {
		return newJSONRPCErrorResponse(req.ID, ErrCodeInvalidParams, errors.ErrMissingParams.Error(), nil), nil
	}

	// Get resource
	registeredResource, exists := m.resources[uri]
	if !exists {
		return newJSONRPCErrorResponse(
			req.ID,
			ErrCodeMethodNotFound,
			fmt.Sprintf("%v: %s", errors.ErrResourceNotFound, uri),
			nil,
		), nil
	}

	// Create resource read request
	readReq := &ReadResourceRequest{
		Params: struct {
			URI       string                 `json:"uri"`
			Arguments map[string]interface{} `json:"arguments,omitempty"`
		}{
			URI: uri,
		},
	}

	// Call resource handler
	contents, err := registeredResource.Handler(ctx, readReq)
	if err != nil {
		return newJSONRPCErrorResponse(req.ID, ErrCodeInternal, err.Error(), nil), nil
	}

	// Create result
	result := ReadResourceResult{
		Contents: contents,
	}

	return result, nil
}

// handleListTemplates handles listing templates requests
func (m *resourceManager) handleListTemplates(ctx context.Context, req *JSONRPCRequest) (JSONRPCMessage, error) {
	templates := m.getTemplates()

	// Convert []*mcp.ResourceTemplate to []mcp.ResourceTemplate for the result
	resultTemplates := make([]ResourceTemplate, len(templates))
	for i, template := range templates {
		resultTemplates[i] = *template
	}

	// Use map structure since ListResourceTemplatesResult might not be defined
	result := map[string]interface{}{
		"resourceTemplates": resultTemplates,
	}

	return result, nil
}

// handleSubscribe handles subscription requests
func (m *resourceManager) handleSubscribe(ctx context.Context, req *JSONRPCRequest) (JSONRPCMessage, error) {
	// Convert params to map for easier access
	paramsMap, ok := req.Params.(map[string]interface{})
	if !ok {
		return newJSONRPCErrorResponse(req.ID, ErrCodeInvalidParams, errors.ErrInvalidParams.Error(), nil), nil
	}

	// Get resource URI from parameters
	uri, ok := paramsMap["uri"].(string)
	if !ok {
		return newJSONRPCErrorResponse(req.ID, ErrCodeInvalidParams, errors.ErrMissingParams.Error(), nil), nil
	}

	// Check if resource exists
	_, exists := m.getResource(uri)
	if !exists {
		return newJSONRPCErrorResponse(req.ID, ErrCodeMethodNotFound, fmt.Sprintf("resource %s not found", uri), nil), nil
	}

	// subscribe to resource updates
	_ = m.subscribe(uri) // We're not using the channel directly in the response

	// Return success response
	result := map[string]interface{}{
		"uri":           uri,
		"subscribeTime": time.Now().UTC().Format(time.RFC3339),
	}

	return result, nil
}

// handleUnsubscribe handles unsubscription requests
func (m *resourceManager) handleUnsubscribe(ctx context.Context, req *JSONRPCRequest) (JSONRPCMessage, error) {
	// Convert params to map for easier access
	paramsMap, ok := req.Params.(map[string]interface{})
	if !ok {
		return newJSONRPCErrorResponse(req.ID, ErrCodeInvalidParams, errors.ErrInvalidParams.Error(), nil), nil
	}

	// Get resource URI from parameters
	uri, ok := paramsMap["uri"].(string)
	if !ok {
		return newJSONRPCErrorResponse(req.ID, ErrCodeInvalidParams, errors.ErrMissingParams.Error(), nil), nil
	}

	// unsubscribe from resource updates
	// Note: In real implementation, you need to locate the specific channel to unsubscribe
	// This is just a simplified implementation

	// Return success response
	result := map[string]interface{}{
		"uri":             uri,
		"unsubscribeTime": time.Now().UTC().Format(time.RFC3339),
	}

	return result, nil
}
