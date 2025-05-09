package protocol

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/modelcontextprotocol/streamable-mcp/schema"
)

// ResourceManager manages resources
//
// Resource functionality follows these enabling mechanisms:
// 1. By default, resource functionality is disabled
// 2. When the first resource is registered, resource functionality is automatically enabled without additional configuration
// 3. When resource functionality is enabled but no resources exist, ListResources will return an empty resource list rather than an error
// 4. Clients can determine if the server supports resource functionality through the capabilities field in the initialization response
//
// This design simplifies API usage, eliminating the need for explicit configuration parameters to enable or disable resource functionality.
type ResourceManager struct {
	// Resource mapping table
	resources map[string]*schema.Resource

	// Resource template mapping table
	templates map[string]*schema.ResourceTemplate

	// Mutex
	mu sync.RWMutex

	// Subscriber mapping table
	subscribers map[string][]chan *schema.Notification

	// Subscriber mutex
	subMu sync.RWMutex
}

// NewResourceManager creates a new resource manager
//
// Note: Simply creating a resource manager does not enable resource functionality,
// it is only enabled when the first resource is added.
func NewResourceManager() *ResourceManager {
	return &ResourceManager{
		resources:   make(map[string]*schema.Resource),
		templates:   make(map[string]*schema.ResourceTemplate),
		subscribers: make(map[string][]chan *schema.Notification),
	}
}

// RegisterResource registers a resource
func (m *ResourceManager) RegisterResource(resource *schema.Resource) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if resource == nil {
		return fmt.Errorf("resource cannot be nil")
	}

	if resource.Name == "" {
		return fmt.Errorf("resource name cannot be empty")
	}

	if resource.URI == "" {
		return fmt.Errorf("resource URI cannot be empty")
	}

	if _, exists := m.resources[resource.URI]; exists {
		return fmt.Errorf("resource %s already exists", resource.URI)
	}

	m.resources[resource.URI] = resource
	return nil
}

// RegisterTemplate registers a resource template
func (m *ResourceManager) RegisterTemplate(template *schema.ResourceTemplate) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if template == nil {
		return fmt.Errorf("template cannot be nil")
	}

	if template.Name == "" {
		return fmt.Errorf("template name cannot be empty")
	}

	if template.URITemplate == "" {
		return fmt.Errorf("template URI cannot be empty")
	}

	if _, exists := m.templates[template.Name]; exists {
		return fmt.Errorf("template %s already exists", template.Name)
	}

	m.templates[template.Name] = template
	return nil
}

// GetResource retrieves a resource
func (m *ResourceManager) GetResource(uri string) (*schema.Resource, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	resource, exists := m.resources[uri]
	return resource, exists
}

// GetResources retrieves all resources
func (m *ResourceManager) GetResources() []*schema.Resource {
	m.mu.RLock()
	defer m.mu.RUnlock()

	resources := make([]*schema.Resource, 0, len(m.resources))
	for _, resource := range m.resources {
		resources = append(resources, resource)
	}
	return resources
}

// GetTemplates retrieves all resource templates
func (m *ResourceManager) GetTemplates() []*schema.ResourceTemplate {
	m.mu.RLock()
	defer m.mu.RUnlock()

	templates := make([]*schema.ResourceTemplate, 0, len(m.templates))
	for _, template := range m.templates {
		templates = append(templates, template)
	}
	return templates
}

// Subscribe subscribes to resource updates
func (m *ResourceManager) Subscribe(uri string) chan *schema.Notification {
	m.subMu.Lock()
	defer m.subMu.Unlock()

	ch := make(chan *schema.Notification, 10)
	m.subscribers[uri] = append(m.subscribers[uri], ch)
	return ch
}

// Unsubscribe cancels a subscription
func (m *ResourceManager) Unsubscribe(uri string, ch chan *schema.Notification) {
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

// NotifyUpdate notifies about resource updates
func (m *ResourceManager) NotifyUpdate(uri string) {
	m.subMu.RLock()
	subs := m.subscribers[uri]
	m.subMu.RUnlock()

	notification := schema.NewNotification("notifications/resources/updated", map[string]interface{}{
		"uri": uri,
	})

	for _, ch := range subs {
		select {
		case ch <- notification:
		default:
			// Skip this subscriber if the channel is full
		}
	}
}

// HandleListResources handles listing resources requests
func (m *ResourceManager) HandleListResources(ctx context.Context, req *schema.Request) (*schema.Response, error) {
	resources := m.GetResources()
	result := &schema.ListResourcesResponse{
		Resources: make([]schema.Resource, len(resources)),
	}

	for i, resource := range resources {
		result.Resources[i] = *resource
	}

	return schema.NewResponse(req.ID, result), nil
}

// HandleReadResource handles reading resource requests
func (m *ResourceManager) HandleReadResource(ctx context.Context, req *schema.Request) (*schema.Response, error) {
	// Get resource URI from parameters
	params := req.Params
	uri, ok := params["uri"].(string)
	if !ok {
		return schema.NewErrorResponse(req.ID, schema.ErrInvalidParams, "missing resource URI", nil), nil
	}

	// Get resource
	resource, exists := m.GetResource(uri)
	if !exists {
		return schema.NewErrorResponse(req.ID, schema.ErrMethodNotFound, fmt.Sprintf("resource %s not found", uri), nil), nil
	}

	// Return resource content - just return the resource directly
	// The response serialization will handle the structure correctly
	return schema.NewResponse(req.ID, resource), nil
}

// HandleListTemplates handles listing templates requests
func (m *ResourceManager) HandleListTemplates(ctx context.Context, req *schema.Request) (*schema.Response, error) {
	templates := m.GetTemplates()
	result := map[string]interface{}{
		"templates": templates,
	}

	return schema.NewResponse(req.ID, result), nil
}

// HandleSubscribe handles subscription requests
func (m *ResourceManager) HandleSubscribe(ctx context.Context, req *schema.Request) (*schema.Response, error) {
	// Get resource URI from parameters
	params := req.Params
	uri, ok := params["uri"].(string)
	if !ok {
		return schema.NewErrorResponse(req.ID, schema.ErrInvalidParams, "missing resource URI", nil), nil
	}

	// Check if resource exists
	_, exists := m.GetResource(uri)
	if !exists {
		return schema.NewErrorResponse(req.ID, schema.ErrMethodNotFound, fmt.Sprintf("resource %s not found", uri), nil), nil
	}

	// Subscribe to resource updates
	_ = m.Subscribe(uri) // We're not using the channel directly in the response

	// Return success response
	result := map[string]interface{}{
		"uri":           uri,
		"subscribeTime": time.Now().UTC().Format(time.RFC3339),
	}

	return schema.NewResponse(req.ID, result), nil
}

// HandleUnsubscribe handles unsubscription requests
func (m *ResourceManager) HandleUnsubscribe(ctx context.Context, req *schema.Request) (*schema.Response, error) {
	// Get resource URI from parameters
	params := req.Params
	uri, ok := params["uri"].(string)
	if !ok {
		return schema.NewErrorResponse(req.ID, schema.ErrInvalidParams, "missing resource URI", nil), nil
	}

	// Unsubscribe from resource updates
	// Note: In real implementation, you need to locate the specific channel to unsubscribe
	// This is just a simplified implementation

	// Return success response
	result := map[string]interface{}{
		"uri":             uri,
		"unsubscribeTime": time.Now().UTC().Format(time.RFC3339),
	}

	return schema.NewResponse(req.ID, result), nil
}
