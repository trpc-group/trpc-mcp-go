package transport

import (
	"context"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"trpc.group/trpc-go/trpc-mcp-go/mcp"
)

// MockHTTPTransport is a mock implementation of the HTTPTransport interface
type MockHTTPTransport struct {
	sendRequestFunc      func(ctx context.Context, req *mcp.JSONRPCRequest) (*mcp.JSONRPCResponse, error)
	sendNotificationFunc func(ctx context.Context, notification *mcp.JSONRPCNotification) error
	closeFunc            func() error
	getSessionIDFunc     func() string
	terminateSessionFunc func(ctx context.Context) error
}

func (m *MockHTTPTransport) SendRequest(ctx context.Context, req *mcp.JSONRPCRequest) (*mcp.JSONRPCResponse, error) {
	if m.sendRequestFunc != nil {
		return m.sendRequestFunc(ctx, req)
	}
	return nil, nil
}

func (m *MockHTTPTransport) SendNotification(ctx context.Context, notification *mcp.JSONRPCNotification) error {
	if m.sendNotificationFunc != nil {
		return m.sendNotificationFunc(ctx, notification)
	}
	return nil
}

func (m *MockHTTPTransport) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

func (m *MockHTTPTransport) GetSessionID() string {
	if m.getSessionIDFunc != nil {
		return m.getSessionIDFunc()
	}
	return ""
}

func (m *MockHTTPTransport) TerminateSession(ctx context.Context) error {
	if m.terminateSessionFunc != nil {
		return m.terminateSessionFunc(ctx)
	}
	return nil
}

func TestMockHTTPTransport(t *testing.T) {
	// Create mock transport with custom behaviors
	mockTransport := &MockHTTPTransport{
		sendRequestFunc: func(ctx context.Context, req *mcp.JSONRPCRequest) (*mcp.JSONRPCResponse, error) {
			// Verify request
			assert.Equal(t, "test.method", req.Method)

			// Return mock response
			return &mcp.JSONRPCResponse{
				JSONRPC: mcp.JSONRPCVersion,
				ID:      req.ID,
				Result:  "test result",
			}, nil
		},
		sendNotificationFunc: func(ctx context.Context, notification *mcp.JSONRPCNotification) error {
			// Verify notification
			assert.Equal(t, "test.notification", notification.Method)
			return nil
		},
		closeFunc: func() error {
			return nil
		},
		getSessionIDFunc: func() string {
			return "test-session-id"
		},
		terminateSessionFunc: func(ctx context.Context) error {
			return nil
		},
	}

	// Test SendRequest
	ctx := context.Background()
	req := mcp.NewJSONRPCRequest(1, "test.method", nil)
	resp, err := mockTransport.SendRequest(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "test result", resp.Result)

	// Test SendNotification
	notification := mcp.NewJSONRPCNotificationFromMap("test.notification", nil)
	err = mockTransport.SendNotification(ctx, notification)
	assert.NoError(t, err)

	// Test GetSessionID
	sessionID := mockTransport.GetSessionID()
	assert.Equal(t, "test-session-id", sessionID)

	// Test TerminateSession
	err = mockTransport.TerminateSession(ctx)
	assert.NoError(t, err)

	// Test Close
	err = mockTransport.Close()
	assert.NoError(t, err)
}

func TestNewStreamableHTTPClientTransport(t *testing.T) {
	// Test creating transport object
	serverURL, _ := url.Parse("http://localhost:3000/mcp")
	transport := NewStreamableHTTPClientTransport(serverURL)

	// Verify object was created successfully
	assert.NotNil(t, transport)

	// Verify basic properties
	assert.Equal(t, "", transport.sessionID) // Initial session ID should be empty
}
