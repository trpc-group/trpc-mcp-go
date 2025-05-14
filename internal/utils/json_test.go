package utils

import (
	"encoding/json"
	"testing"
)

func TestParseJSONObject(t *testing.T) {
	// Test case.
	testCases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "Valid JSON Object",
			input:   `{"name":"test","value":123}`,
			wantErr: false,
		},
		{
			name:    "Invalid JSON",
			input:   `{"name":"test"`,
			wantErr: true,
		},
		{
			name:    "Empty Object",
			input:   `{}`,
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rawMsg := json.RawMessage(tc.input)
			result, err := ParseJSONObject(&rawMsg)

			if tc.wantErr && err == nil {
				t.Errorf("ParseJSONObject() error = nil, wantErr %v", tc.wantErr)
				return
			}

			if !tc.wantErr && err != nil {
				t.Errorf("ParseJSONObject() error = %v, wantErr %v", err, tc.wantErr)
				return
			}

			if !tc.wantErr && result == nil {
				t.Errorf("ParseJSONObject() result is nil, expected non-nil")
			}
		})
	}
}

func TestExtractString(t *testing.T) {
	// Test data.
	data := map[string]interface{}{
		"string_key": "test_value",
		"int_key":    123,
		"bool_key":   true,
	}

	// Test case.
	testCases := []struct {
		name     string
		key      string
		expected string
	}{
		{
			name:     "Existing String Key",
			key:      "string_key",
			expected: "test_value",
		},
		{
			name:     "Non-String Key",
			key:      "int_key",
			expected: "",
		},
		{
			name:     "Non-Existent Key",
			key:      "missing_key",
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ExtractString(data, tc.key)

			if result != tc.expected {
				t.Errorf("ExtractString() = %v, expected %v", result, tc.expected)
			}
		})
	}
}

func TestExtractMap(t *testing.T) {
	// Test data.
	nestedMap := map[string]interface{}{
		"key1": "value1",
	}
	data := map[string]interface{}{
		"map_key":    nestedMap,
		"string_key": "test_value",
	}

	// Test case.
	testCases := []struct {
		name     string
		key      string
		expected map[string]interface{}
	}{
		{
			name:     "Existing Map Key",
			key:      "map_key",
			expected: nestedMap,
		},
		{
			name:     "Non-Map Key",
			key:      "string_key",
			expected: nil,
		},
		{
			name:     "Non-Existent Key",
			key:      "missing_key",
			expected: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ExtractMap(data, tc.key)

			if tc.expected == nil && result != nil {
				t.Errorf("ExtractMap() = %v, expected nil", result)
				return
			}

			if tc.expected != nil && result == nil {
				t.Errorf("ExtractMap() = nil, expected non-nil")
				return
			}

			if tc.expected != nil && result != nil {
				// Simply check a key-value pair.
				if tc.expected["key1"] != result["key1"] {
					t.Errorf("ExtractMap() values don't match, expected %v, got %v", tc.expected, result)
				}
			}
		})
	}
}
