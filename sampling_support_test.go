package mcp

import (
	"context"
	"fmt"
	"testing"
	"time"

	"trpc.group/trpc-go/trpc-mcp-go/sampling"
)

// ===============================================
// 测试工具和Mock对象
// ===============================================

// MockSamplingHandler - Simulated Sampling Processor
type MockSamplingHandler struct {
	CallCount    int
	LastRequest  *sampling.SamplingCreateMessageRequest
	ReturnResult *SamplingCreateMessageResult
	ReturnError  error
	ShouldFail   bool
}

func (m *MockSamplingHandler) HandleSamplingRequest(ctx context.Context, req *sampling.SamplingCreateMessageRequest) (*SamplingCreateMessageResult, error) {
	m.CallCount++
	m.LastRequest = req

	if m.ShouldFail {
		return nil, m.ReturnError
	}

	if m.ReturnResult != nil {
		return m.ReturnResult, nil
	}

	return &SamplingCreateMessageResult{
		Role: "assistant",
		Content: SamplingTextContent{
			Type: "text",
			Text: "Mock response",
		},
		Model:      "mock-model",
		StopReason: "stop",
	}, nil
}

// createTestConfig - Creating a Test Configuration
func createTestConfig() *SamplingClientConfig {
	return &SamplingClientConfig{
		DefaultModel:        "test-model",
		AutoApprove:         false,
		MaxTokensPerRequest: 1000,
		ModelMappings: map[string]string{
			"test-hint": "mapped-model",
		},
		TimeoutSeconds: 30,
	}
}

// createTestServerConfig - Creating a test server configuration
func createTestServerConfig() *SamplingServerConfig {
	return &SamplingServerConfig{
		MaxTokensLimit:      2000,
		RateLimitPerMinute:  10,
		AllowedContentTypes: []string{"text"},
		RequireApproval:     true,
	}
}

// ===============================================
//Tool function test
// ===============================================

func TestUtilityFunctions(t *testing.T) {
	t.Run("Pointer utility functions", func(t *testing.T) {
		intVal := 42
		intPtr := IntPtr(intVal)
		if intPtr == nil || *intPtr != intVal {
			t.Errorf("IntPtr failed: expected %d, got %v", intVal, intPtr)
		}

		floatVal := 3.14
		floatPtr := FloatPtr(floatVal)
		if floatPtr == nil || *floatPtr != floatVal {
			t.Errorf("FloatPtr failed: expected %f, got %v", floatVal, floatPtr)
		}

		stringVal := "test"
		stringPtr := StringPtr(stringVal)
		if stringPtr == nil || *stringPtr != stringVal {
			t.Errorf("StringPtr failed: expected %s, got %v", stringVal, stringPtr)
		}
	})

	t.Run("Request ID Generator", func(t *testing.T) {
		id1 := GenerateRequestID()
		time.Sleep(1 * time.Microsecond)
		id2 := GenerateRequestID()

		if id1 == 0 || id2 == 0 {
			t.Error("GenerateRequestID returned zero value")
		}

		if id1 == id2 {
			t.Error("GenerateRequestID should generate unique IDs")
		}
	})

	t.Run("String Contains Check", func(t *testing.T) {
		slice := []string{"apple", "banana", "cherry"}

		if !containsString(slice, "banana") {
			t.Error("containsString failed to find existing item")
		}

		if containsString(slice, "grape") {
			t.Error("containsString incorrectly found non-existing item")
		}

		if containsString([]string{}, "test") {
			t.Error("containsString should return false for empty slice")
		}
	})
}

// ===============================================
//Sampling request verification test
// ===============================================

func TestValidateSamplingRequest(t *testing.T) {
	tests := []struct {
		name    string
		request *sampling.SamplingCreateMessageRequest
		wantErr bool
		errMsg  string
	}{
		{
			name:    "nil request",
			request: nil,
			wantErr: true,
			errMsg:  "request is nil",
		},
		{
			name: "Invalid method",
			request: &sampling.SamplingCreateMessageRequest{
				Method: "invalid/method",
			},
			wantErr: true,
			errMsg:  "invalid method: invalid/method",
		},
		{
			name: "Empty message",
			request: &sampling.SamplingCreateMessageRequest{
				Method: "sampling/createMessage",
				Params: sampling.SamplingCreateMessageParams{
					Messages: []sampling.SamplingMessage{},
				},
			},
			wantErr: true,
			errMsg:  "messages cannot be empty",
		},
		{
			name: "Invalid role",
			request: &sampling.SamplingCreateMessageRequest{
				Method: "sampling/createMessage",
				Params: sampling.SamplingCreateMessageParams{
					Messages: []sampling.SamplingMessage{
						{
							Role: "invalid_role",
							Content: SamplingTextContent{
								Type: "text",
								Text: "test",
							},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "message 0: invalid role 'invalid_role'",
		},
		{
			name: "Empty Role",
			request: &sampling.SamplingCreateMessageRequest{
				Method: "sampling/createMessage",
				Params: sampling.SamplingCreateMessageParams{
					Messages: []sampling.SamplingMessage{
						{
							Role: "",
							Content: SamplingTextContent{
								Type: "text",
								Text: "test",
							},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "message 0: role cannot be empty",
		},
		{
			name: "nil content",
			request: &sampling.SamplingCreateMessageRequest{
				Method: "sampling/createMessage",
				Params: sampling.SamplingCreateMessageParams{
					Messages: []sampling.SamplingMessage{
						{
							Role:    "user",
							Content: nil,
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "message 0: content cannot be nil",
		},
		{
			name: "Valid Request",
			request: &sampling.SamplingCreateMessageRequest{
				Method: "sampling/createMessage",
				Params: sampling.SamplingCreateMessageParams{
					Messages: []sampling.SamplingMessage{
						{
							Role: "user",
							Content: SamplingTextContent{
								Type: "text",
								Text: "test message",
							},
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSamplingRequest(tt.request)
			if tt.wantErr {
				if err == nil {
					t.Errorf("validateSamplingRequest() expected error, got nil")
				} else if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("validateSamplingRequest() error = %v, want %v", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("validateSamplingRequest() unexpected error = %v", err)
				}
			}
		})
	}
}

// ===============================================
//Default Sampling Processor Test
// ===============================================

func TestDefaultSamplingHandler(t *testing.T) {
	t.Run("Creating a default handler", func(t *testing.T) {
		handler1 := NewDefaultSamplingHandler(nil)
		if handler1 == nil {
			t.Error("NewDefaultSamplingHandler should not return nil")
		}

		config := createTestConfig()
		handler2 := NewDefaultSamplingHandler(config)
		if handler2 == nil {
			t.Error("NewDefaultSamplingHandler should not return nil")
		}

		if _, ok := handler1.(*DefaultSamplingHandler); !ok {
			t.Error("NewDefaultSamplingHandler should return *DefaultSamplingHandler")
		}
	})

	t.Run("Processing sampling requests", func(t *testing.T) {
		config := createTestConfig()
		handler := NewDefaultSamplingHandler(config)

		req := &sampling.SamplingCreateMessageRequest{
			Method: "sampling/createMessage",
			Params: sampling.SamplingCreateMessageParams{
				Messages: []sampling.SamplingMessage{
					{
						Role: "user",
						Content: SamplingTextContent{
							Type: "text",
							Text: "Test Message",
						},
					},
				},
				ModelPreferences: &sampling.ModelPreferences{
					Hints: []string{"test-hint"},
				},
			},
		}

		ctx := context.Background()
		result, err := handler.HandleSamplingRequest(ctx, req)

		if err != nil {
			t.Errorf("HandleSamplingRequest failed: %v", err)
		}

		if result == nil {
			t.Error("HandleSamplingRequest returned nil result")
		}

		if result.Role != "assistant" {
			t.Errorf("Expected role 'assistant', got '%s'", result.Role)
		}

		if result.Model != "mapped-model" {
			t.Errorf("Expected model 'mapped-model', got '%s'", result.Model)
		}
	})

	t.Run("Model Mapping Test", func(t *testing.T) {
		config := &SamplingClientConfig{
			DefaultModel: "default-model",
			ModelMappings: map[string]string{
				"claude-3-sonnet": "gpt-4",
				"claude-3-haiku":  "gpt-3.5-turbo",
			},
		}
		handler := NewDefaultSamplingHandler(config)

		tests := []struct {
			name     string
			hints    []string
			expected string
		}{
			{
				name:     "No prompts",
				hints:    nil,
				expected: "default-model",
			},
			{
				name:     "Mapping prompts",
				hints:    []string{"claude-3-sonnet"},
				expected: "gpt-4",
			},
			{
				name:     "Unmapped prompts",
				hints:    []string{"unknown-model"},
				expected: "default-model",
			},
			{
				name:     "Multiple prompts, first mapping",
				hints:    []string{"claude-3-haiku", "claude-3-sonnet"},
				expected: "gpt-3.5-turbo",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				req := &sampling.SamplingCreateMessageRequest{
					Method: "sampling/createMessage",
					Params: sampling.SamplingCreateMessageParams{
						Messages: []sampling.SamplingMessage{
							{
								Role: "user",
								Content: SamplingTextContent{
									Type: "text",
									Text: "test",
								},
							},
						},
					},
				}

				if tt.hints != nil {
					req.Params.ModelPreferences = &sampling.ModelPreferences{
						Hints: tt.hints,
					}
				}

				result, err := handler.HandleSamplingRequest(context.Background(), req)
				if err != nil {
					t.Errorf("HandleSamplingRequest failed: %v", err)
				}

				if result.Model != tt.expected {
					t.Errorf("Expected model '%s', got '%s'", tt.expected, result.Model)
				}
			})
		}
	})
}

// ===============================================
//OpenAI Sampling Processor Test
// ===============================================

func TestOpenAISamplingHandler(t *testing.T) {
	t.Run("Creating the OpenAI Processor", func(t *testing.T) {
		handler := NewOpenAISamplingHandler("test-api-key", nil)
		if handler == nil {
			t.Error("NewOpenAISamplingHandler should not return nil")
		}

		if _, ok := handler.(*OpenAISamplingHandler); !ok {
			t.Error("NewOpenAISamplingHandler should return *OpenAISamplingHandler")
		}
	})

	t.Run("Processing sampling requests", func(t *testing.T) {
		handler := NewOpenAISamplingHandler("test-api-key", createTestConfig())

		req := &sampling.SamplingCreateMessageRequest{
			Method: "sampling/createMessage",
			Params: sampling.SamplingCreateMessageParams{
				Messages: []sampling.SamplingMessage{
					{
						Role: "user",
						Content: SamplingTextContent{
							Type: "text",
							Text: "测试OpenAI处理器",
						},
					},
				},
			},
		}

		result, err := handler.HandleSamplingRequest(context.Background(), req)
		if err != nil {
			t.Errorf("HandleSamplingRequest failed: %v", err)
		}

		if result == nil {
			t.Error("HandleSamplingRequest returned nil result")
		}

		if result.Role != "assistant" {
			t.Errorf("Expected role 'assistant', got '%s'", result.Role)
		}

		if result.Usage == nil {
			t.Error("Expected Usage information, got nil")
		}
	})

	t.Run("Model selection test", func(t *testing.T) {
		config := &SamplingClientConfig{
			DefaultModel: "gpt-3.5-turbo",
			ModelMappings: map[string]string{
				"claude-3-sonnet": "gpt-4",
			},
		}
		handler := NewOpenAISamplingHandler("test-api-key", config).(*OpenAISamplingHandler)

		tests := []struct {
			name     string
			prefs    *sampling.ModelPreferences
			expected string
		}{
			{
				name:     "nil preference",
				prefs:    nil,
				expected: "gpt-3.5-turbo",
			},
			{
				name: "High intelligence priority",
				prefs: &sampling.ModelPreferences{
					IntelligencePriority: FloatPtr(0.9),
				},
				expected: "gpt-4",
			},
			{
				name: "High speed priority",
				prefs: &sampling.ModelPreferences{
					SpeedPriority: FloatPtr(0.9),
				},
				expected: "gpt-3.5-turbo",
			},
			{
				name: "High cost priority",
				prefs: &sampling.ModelPreferences{
					CostPriority: FloatPtr(0.9),
				},
				expected: "gpt-3.5-turbo",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := handler.selectModel(tt.prefs)
				if result != tt.expected {
					t.Errorf("Expected model '%s', got '%s'", tt.expected, result)
				}
			})
		}
	})
}

// ===============================================
//Adapter Test
// ===============================================

func TestSamplingHandlerAdapter(t *testing.T) {
	t.Run("Creating an Adapter", func(t *testing.T) {
		mockHandler := &MockSamplingHandler{}
		adapter := NewSamplingHandlerAdapter(mockHandler)

		if adapter == nil {
			t.Error("NewSamplingHandlerAdapter should not return nil")
		}

		// 验证类型
		if _, ok := adapter.(*SamplingHandlerAdapter); !ok {
			t.Error("NewSamplingHandlerAdapter should return *SamplingHandlerAdapter")
		}
	})

	t.Run("Adapter Call", func(t *testing.T) {
		mockHandler := &MockSamplingHandler{
			ReturnResult: &SamplingCreateMessageResult{
				Role:       "assistant",
				Content:    SamplingTextContent{Type: "text", Text: "adapter test"},
				Model:      "adapter-model",
				StopReason: "stop",
			},
		}

		adapter := NewSamplingHandlerAdapter(mockHandler)

		req := &sampling.SamplingCreateMessageRequest{
			Method: "sampling/createMessage",
			Params: sampling.SamplingCreateMessageParams{
				Messages: []sampling.SamplingMessage{
					{
						Role: "user",
						Content: SamplingTextContent{
							Type: "text",
							Text: "adapter test",
						},
					},
				},
			},
		}

		result, err := adapter.HandleSamplingRequest(context.Background(), req)
		if err != nil {
			t.Errorf("Adapter HandleSamplingRequest failed: %v", err)
		}

		if mockHandler.CallCount != 1 {
			t.Errorf("Expected mock handler to be called once, got %d", mockHandler.CallCount)
		}

		if result.Model != "adapter-model" {
			t.Errorf("Expected model 'adapter-model', got '%s'", result.Model)
		}
	})

	t.Run("Adapter Error Handling", func(t *testing.T) {
		mockHandler := &MockSamplingHandler{
			ShouldFail:  true,
			ReturnError: fmt.Errorf("mock error"),
		}

		adapter := NewSamplingHandlerAdapter(mockHandler)

		req := &sampling.SamplingCreateMessageRequest{
			Method: "sampling/createMessage",
			Params: sampling.SamplingCreateMessageParams{
				Messages: []sampling.SamplingMessage{
					{
						Role: "user",
						Content: SamplingTextContent{
							Type: "text",
							Text: "error test",
						},
					},
				},
			},
		}

		_, err := adapter.HandleSamplingRequest(context.Background(), req)
		if err == nil {
			t.Error("Expected adapter to return error, got nil")
		}

		if err.Error() != "mock error" {
			t.Errorf("Expected error 'mock error', got '%s'", err.Error())
		}
	})

	t.Run("Invalid Handler", func(t *testing.T) {
		// 测试非指针类型
		adapter := NewSamplingHandlerAdapter("not a pointer")

		req := &sampling.SamplingCreateMessageRequest{
			Method: "sampling/createMessage",
			Params: sampling.SamplingCreateMessageParams{
				Messages: []sampling.SamplingMessage{
					{
						Role:    "user",
						Content: SamplingTextContent{Type: "text", Text: "test"},
					},
				},
			},
		}

		_, err := adapter.HandleSamplingRequest(context.Background(), req)
		if err == nil {
			t.Error("Expected adapter to return error for non-pointer handler")
		}
	})
}

// ===============================================
//Client Sampling Function Test
// ===============================================

func TestClientSamplingSupport(t *testing.T) {
	t.Run("WithSamplingHandler选项", func(t *testing.T) {
		client := &Client{}
		handler := NewDefaultSamplingHandler(nil)

		option := WithSamplingHandler(handler)
		option(client)

		support := ClientSamplingMap[client]
		if support == nil {
			t.Error("Expected client sampling support to be created")
		}

		if !support.SamplingEnabled {
			t.Error("Expected sampling to be enabled")
		}

		if support.SamplingHandler != handler {
			t.Error("Expected handler to be set correctly")
		}
	})

	t.Run("WithSamplingConfig option", func(t *testing.T) {
		client := &Client{}
		config := createTestConfig()

		option := WithSamplingConfig(config)
		option(client)

		support := ClientSamplingMap[client]
		if support == nil {
			t.Error("Expected client sampling support to be created")
		}

		if !support.SamplingEnabled {
			t.Error("Expected sampling to be enabled")
		}

		if support.samplingConfig != config {
			t.Error("Expected config to be set correctly")
		}
	})

	t.Run("IsSamplingEnabled", func(t *testing.T) {
		client := &Client{}

		if client.IsSamplingEnabled() {
			t.Error("Expected sampling to be disabled initially")
		}

		option := WithSamplingHandler(NewDefaultSamplingHandler(nil))
		option(client)

		if !client.IsSamplingEnabled() {
			t.Error("Expected sampling to be enabled after setting handler")
		}
	})

	t.Run("GetSamplingConfig", func(t *testing.T) {
		client := &Client{}

		config := client.GetSamplingConfig()
		if config != nil {
			t.Error("Expected nil config for unconfigured client")
		}

		testConfig := createTestConfig()
		option := WithSamplingConfig(testConfig)
		option(client)

		config = client.GetSamplingConfig()
		if config != testConfig {
			t.Error("Expected config to match what was set")
		}
	})

	t.Run("HandleSamplingRequest", func(t *testing.T) {
		client := &Client{}
		mockHandler := &MockSamplingHandler{
			ReturnResult: &SamplingCreateMessageResult{
				Role:       "assistant",
				Content:    SamplingTextContent{Type: "text", Text: "client test"},
				Model:      "client-model",
				StopReason: "stop",
			},
		}

		WithSamplingHandler(mockHandler)(client)
		WithSamplingConfig(createTestConfig())(client)

		req := &sampling.SamplingCreateMessageRequest{
			Method: "sampling/createMessage",
			Params: sampling.SamplingCreateMessageParams{
				Messages: []sampling.SamplingMessage{
					{
						Role: "user",
						Content: SamplingTextContent{
							Type: "text",
							Text: "client test",
						},
					},
				},
				MaxTokens: IntPtr(500),
			},
		}

		result, err := client.HandleSamplingRequest(context.Background(), req)
		if err != nil {
			t.Errorf("Client HandleSamplingRequest failed: %v", err)
		}

		if result.Model != "client-model" {
			t.Errorf("Expected model 'client-model', got '%s'", result.Model)
		}

		if mockHandler.CallCount != 1 {
			t.Errorf("Expected handler to be called once, got %d", mockHandler.CallCount)
		}
	})

	t.Run("HandleSamplingRequest错误情况", func(t *testing.T) {
		tests := []struct {
			name          string
			setupClient   func(*Client)
			maxTokens     *int
			expectedError string
		}{
			{
				name: "Sampling is not enabled",
				setupClient: func(c *Client) {
				},
				expectedError: "sampling not enabled",
			},
			{
				name: "Processor not configured",
				setupClient: func(c *Client) {
					WithSamplingConfig(createTestConfig())(c)
				},
				expectedError: "sampling handler not configured",
			},
			{
				name: "Exceeding the Token Limit",
				setupClient: func(c *Client) {
					WithSamplingHandler(NewDefaultSamplingHandler(nil))(c)
					WithSamplingConfig(createTestConfig())(c)
				},
				maxTokens:     IntPtr(2000),
				expectedError: "max tokens (2000) exceeds limit (1000)",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				client := &Client{}
				tt.setupClient(client)

				req := &sampling.SamplingCreateMessageRequest{
					Method: "sampling/createMessage",
					Params: sampling.SamplingCreateMessageParams{
						Messages: []sampling.SamplingMessage{
							{
								Role: "user",
								Content: SamplingTextContent{
									Type: "text",
									Text: "error test",
								},
							},
						},
						MaxTokens: tt.maxTokens,
					},
				}

				_, err := client.HandleSamplingRequest(context.Background(), req)
				if err == nil {
					t.Errorf("Expected error, got nil")
				}

				if err.Error() != tt.expectedError {
					t.Errorf("Expected error '%s', got '%s'", tt.expectedError, err.Error())
				}
			})
		}
	})

	t.Run("Timeout handling", func(t *testing.T) {
		client := &Client{}

		slowHandler := &MockSamplingHandler{}
		slowHandler.ReturnResult = &SamplingCreateMessageResult{
			Role:       "assistant",
			Content:    SamplingTextContent{Type: "text", Text: "slow response"},
			Model:      "slow-model",
			StopReason: "stop",
		}

		config := &SamplingClientConfig{
			DefaultModel:        "test-model",
			MaxTokensPerRequest: 1000,
			TimeoutSeconds:      1,
		}

		WithSamplingHandler(slowHandler)(client)
		WithSamplingConfig(config)(client)

		req := &sampling.SamplingCreateMessageRequest{
			Method: "sampling/createMessage",
			Params: sampling.SamplingCreateMessageParams{
				Messages: []sampling.SamplingMessage{
					{
						Role: "user",
						Content: SamplingTextContent{
							Type: "text",
							Text: "timeout test",
						},
					},
				},
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		_, err := client.HandleSamplingRequest(ctx, req)

		if err != nil && err != context.DeadlineExceeded {
			t.Logf("Context timeout test completed, error: %v", err)
		}
	})
}

// ===============================================
//Server Sampling Function Test
// ===============================================

func TestServerSamplingSupport(t *testing.T) {
	t.Run("WithSamplingEnabled选项", func(t *testing.T) {
		server := &Server{}

		option := WithSamplingEnabled(true)
		option(server)

		support := ServerSamplingMap[server]
		if support == nil {
			t.Error("Expected server sampling support to be created")
		}

		if !support.SamplingEnabled {
			t.Error("Expected sampling to be enabled")
		}

		if !server.IsSamplingEnabled() {
			t.Error("Expected IsSamplingEnabled to return true")
		}
	})

	t.Run("WithSamplingConfigServer选项", func(t *testing.T) {
		server := &Server{}
		config := createTestServerConfig()

		option := WithSamplingConfigServer(config)
		option(server)

		support := ServerSamplingMap[server]
		if support == nil {
			t.Error("Expected server sampling support to be created")
		}

		if !support.SamplingEnabled {
			t.Error("Expected sampling to be enabled")
		}

		if support.samplingConfig != config {
			t.Error("Expected config to be set correctly")
		}

		retrievedConfig := server.GetSamplingConfig()
		if retrievedConfig != config {
			t.Error("Expected GetSamplingConfig to return the set config")
		}
	})

	t.Run("RegisterSamplingHandler", func(t *testing.T) {
		server := &Server{}
		handler := NewDefaultSamplingHandler(nil)

		server.RegisterSamplingHandler(handler)

		support := ServerSamplingMap[server]
		if support == nil {
			t.Error("Expected server sampling support to be created")
		}

		if !support.SamplingEnabled {
			t.Error("Expected sampling to be enabled")
		}

		if support.SamplingHandler != handler {
			t.Error("Expected handler to be set correctly")
		}
	})

	t.Run("SendSamplingRequest", func(t *testing.T) {
		server := &Server{}
		mockHandler := &MockSamplingHandler{
			ReturnResult: &SamplingCreateMessageResult{
				Role:       "assistant",
				Content:    SamplingTextContent{Type: "text", Text: "server test"},
				Model:      "server-model",
				StopReason: "stop",
			},
		}

		server.RegisterSamplingHandler(mockHandler)

		req := &sampling.SamplingCreateMessageRequest{
			Method: "sampling/createMessage",
			Params: sampling.SamplingCreateMessageParams{
				Messages: []sampling.SamplingMessage{
					{
						Role: "user",
						Content: SamplingTextContent{
							Type: "text",
							Text: "server test",
						},
					},
				},
			},
		}

		result, err := server.SendSamplingRequest(context.Background(), req)
		if err != nil {
			t.Errorf("Server SendSamplingRequest failed: %v", err)
		}

		if result.Model != "server-model" {
			t.Errorf("Expected model 'server-model', got '%s'", result.Model)
		}

		if mockHandler.CallCount != 1 {
			t.Errorf("Expected handler to be called once, got %d", mockHandler.CallCount)
		}
	})

	t.Run("SendSamplingRequest Error Conditions", func(t *testing.T) {
		tests := []struct {
			name          string
			setupServer   func(*Server)
			expectedError string
		}{
			{
				name: "Sampling未启用",
				setupServer: func(s *Server) {
					// 不启用Sampling
				},
				expectedError: "sampling not enabled",
			},
			{
				name: "Processor not configured",
				setupServer: func(s *Server) {
					WithSamplingEnabled(true)(s)
				},
				expectedError: "sampling handler not configured",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				server := &Server{}
				tt.setupServer(server)

				req := &sampling.SamplingCreateMessageRequest{
					Method: "sampling/createMessage",
					Params: sampling.SamplingCreateMessageParams{
						Messages: []sampling.SamplingMessage{
							{
								Role: "user",
								Content: SamplingTextContent{
									Type: "text",
									Text: "error test",
								},
							},
						},
					},
				}

				_, err := server.SendSamplingRequest(context.Background(), req)
				if err == nil {
					t.Error("Expected error, got nil")
				}

				if err.Error() != tt.expectedError {
					t.Errorf("Expected error '%s', got '%s'", tt.expectedError, err.Error())
				}
			})
		}
	})
}

// ===============================================
//Contextual support testing
// ===============================================

func TestSamplingContext(t *testing.T) {
	t.Run("SetSamplingSender和GetSamplingSender", func(t *testing.T) {
		server := &Server{}
		ctx := context.Background()

		_, ok := GetSamplingSender(ctx)
		if ok {
			t.Error("Expected GetSamplingSender to return false for empty context")
		}

		ctxWithSender := SetSamplingSender(ctx, server)
		sender, ok := GetSamplingSender(ctxWithSender)
		if !ok {
			t.Error("Expected GetSamplingSender to return true after setting")
		}

		if sender != server {
			t.Error("Expected sender to be the server that was set")
		}
	})

	t.Run("Context Chain", func(t *testing.T) {
		server1 := &Server{}
		server2 := &Server{}
		ctx := context.Background()

		ctx1 := SetSamplingSender(ctx, server1)
		sender1, ok1 := GetSamplingSender(ctx1)
		if !ok1 || sender1 != server1 {
			t.Error("Expected first sender to be set correctly")
		}

		ctx2 := SetSamplingSender(ctx1, server2)
		sender2, ok2 := GetSamplingSender(ctx2)
		if !ok2 || sender2 != server2 {
			t.Error("Expected second sender to override first")
		}

		senderOrig, okOrig := GetSamplingSender(ctx1)
		if !okOrig || senderOrig != server1 {
			t.Error("Expected original context to still have first sender")
		}
	})
}

// ===============================================
//Cleanup function test
// ===============================================

func TestCleanupFunctions(t *testing.T) {
	t.Run("CleanupClientSampling", func(t *testing.T) {
		client := &Client{}

		WithSamplingHandler(NewDefaultSamplingHandler(nil))(client)

		if !client.IsSamplingEnabled() {
			t.Error("Expected sampling to be enabled before cleanup")
		}

		CleanupClientSampling(client)

		if client.IsSamplingEnabled() {
			t.Error("Expected sampling to be disabled after cleanup")
		}

		if _, exists := ClientSamplingMap[client]; exists {
			t.Error("Expected client to be removed from sampling map")
		}
	})

	t.Run("CleanupServerSampling", func(t *testing.T) {
		server := &Server{}

		WithSamplingEnabled(true)(server)

		if !server.IsSamplingEnabled() {
			t.Error("Expected sampling to be enabled before cleanup")
		}

		CleanupServerSampling(server)

		if server.IsSamplingEnabled() {
			t.Error("Expected sampling to be disabled after cleanup")
		}

		if _, exists := ServerSamplingMap[server]; exists {
			t.Error("Expected server to be removed from sampling map")
		}
	})
}

// ===============================================
//Convenience Constructor Test
// ===============================================

func TestConvenienceFunctions(t *testing.T) {
	t.Run("NewSamplingHandler", func(t *testing.T) {
		config := createTestConfig()
		handler := NewSamplingHandler(config)

		if handler == nil {
			t.Error("NewSamplingHandler should not return nil")
		}

		defaultHandler := NewDefaultSamplingHandler(config)
		if fmt.Sprintf("%T", handler) != fmt.Sprintf("%T", defaultHandler) {
			t.Error("NewSamplingHandler should return same type as NewDefaultSamplingHandler")
		}
	})

	t.Run("NewOpenAIHandler", func(t *testing.T) {
		config := createTestConfig()
		handler := NewOpenAIHandler("test-api-key", config)

		if handler == nil {
			t.Error("NewOpenAIHandler should not return nil")
		}

		openaiHandler := NewOpenAISamplingHandler("test-api-key", config)
		if fmt.Sprintf("%T", handler) != fmt.Sprintf("%T", openaiHandler) {
			t.Error("NewOpenAIHandler should return same type as NewOpenAISamplingHandler")
		}
	})
}

// ===============================================
//Integration Testing
// ===============================================

func TestSamplingIntegration(t *testing.T) {
	t.Run("End-to-end Sampling Process", func(t *testing.T) {

		server := &Server{}
		WithSamplingEnabled(true)(server)
		WithSamplingConfigServer(createTestServerConfig())(server)

		handler := NewDefaultSamplingHandler(createTestConfig())
		server.RegisterSamplingHandler(handler)

		client := &Client{}
		WithSamplingHandler(handler)(client)
		WithSamplingConfig(createTestConfig())(client)

		//Create a context and set the SamplingSender
		ctx := SetSamplingSender(context.Background(), server)

		//Verify that there is a SamplingSender in the context
		sender, ok := GetSamplingSender(ctx)
		if !ok {
			t.Error("Expected SamplingSender in context")
		}

		// Create sampling request
		req := &sampling.SamplingCreateMessageRequest{
			Method: "sampling/createMessage",
			Params: sampling.SamplingCreateMessageParams{
				Messages: []sampling.SamplingMessage{
					{
						Role: "user",
						Content: SamplingTextContent{
							Type: "text",
							Text: "End-to-end testing",
						},
					},
				},
				MaxTokens: IntPtr(500),
			},
		}

		result, err := sender.SendSamplingRequest(ctx, req)
		if err != nil {
			t.Errorf("End-to-end sampling failed: %v", err)
		}

		if result == nil {
			t.Error("Expected non-nil result")
		}

		if result.Role != "assistant" {
			t.Errorf("Expected role 'assistant', got '%s'", result.Role)
		}

		if textContent, ok := result.Content.(SamplingTextContent); ok {
			if textContent.Text == "" {
				t.Error("Expected non-empty response text")
			}
		} else {
			t.Error("Expected SamplingTextContent")
		}
	})

	t.Run("Multiple client concurrent testing", func(t *testing.T) {
		server := &Server{}
		server.RegisterSamplingHandler(NewDefaultSamplingHandler(nil))

		clients := make([]*Client, 5)
		for i := range clients {
			clients[i] = &Client{}
			WithSamplingHandler(NewDefaultSamplingHandler(nil))(clients[i])
		}

		ctx := SetSamplingSender(context.Background(), server)

		//Concurrency Testing
		done := make(chan bool, len(clients))
		for i, client := range clients {
			go func(clientIndex int, c *Client) {
				defer func() { done <- true }()

				req := &sampling.SamplingCreateMessageRequest{
					Method: "sampling/createMessage",
					Params: sampling.SamplingCreateMessageParams{
						Messages: []sampling.SamplingMessage{
							{
								Role: "user",
								Content: SamplingTextContent{
									Type: "text",
									Text: fmt.Sprintf("并发测试客户端 %d", clientIndex),
								},
							},
						},
					},
				}

				_, err := c.HandleSamplingRequest(ctx, req)
				if err != nil {
					t.Errorf("Client %d failed: %v", clientIndex, err)
				}
			}(i, client)
		}

		for i := 0; i < len(clients); i++ {
			<-done
		}
	})
}

// ===============================================
// Benchmarks
// ===============================================

func BenchmarkSamplingOperations(b *testing.B) {
	b.Run("DefaultSamplingHandler", func(b *testing.B) {
		handler := NewDefaultSamplingHandler(createTestConfig())
		req := &sampling.SamplingCreateMessageRequest{
			Method: "sampling/createMessage",
			Params: sampling.SamplingCreateMessageParams{
				Messages: []sampling.SamplingMessage{
					{
						Role: "user",
						Content: SamplingTextContent{
							Type: "text",
							Text: "benchmark test",
						},
					},
				},
			},
		}

		ctx := context.Background()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_, err := handler.HandleSamplingRequest(ctx, req)
			if err != nil {
				b.Fatalf("Handler failed: %v", err)
			}
		}
	})

	b.Run("GenerateRequestID", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = GenerateRequestID()
		}
	})

	b.Run("ValidateSamplingRequest", func(b *testing.B) {
		req := &sampling.SamplingCreateMessageRequest{
			Method: "sampling/createMessage",
			Params: sampling.SamplingCreateMessageParams{
				Messages: []sampling.SamplingMessage{
					{
						Role: "user",
						Content: SamplingTextContent{
							Type: "text",
							Text: "validation test",
						},
					},
				},
			},
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = validateSamplingRequest(req)
		}
	})
}
