package mcp

import "encoding/json"

// MCP protcol Layer

// Request is the base request struct for all MCP requests.
type Request struct {
	Method string `json:"method"`
	Params struct {
		Meta *struct {
			ProgressToken ProgressToken `json:"progressToken,omitempty"`
		} `json:"_meta,omitempty"`
	} `json:"params,omitempty"`
}

// Notification is the base notification struct for all MCP notifications.
type Notification struct {
	Method string             `json:"method"`
	Params NotificationParams `json:"params,omitempty"`
}

// NotificationParams is the base notification params struct for all MCP notifications.
type NotificationParams struct {
	Meta             map[string]interface{} `json:"_meta,omitempty"`
	AdditionalFields map[string]interface{} `json:"-"` // Additional fields that are not part of the MCP protocol.
}

// Meta represents the _meta field in MCP objects.
// Using map[string]interface{} for flexibility as in mcp-go.
type Meta map[string]interface{}

// MarshalJSON implements custom JSON marshaling for NotificationParams.
// It flattens the AdditionalFields into the main JSON object.
func (p NotificationParams) MarshalJSON() ([]byte, error) {
	m := make(map[string]interface{})

	// Add Meta if it exists and is not empty
	if p.Meta != nil && len(p.Meta) > 0 {
		m["_meta"] = p.Meta
	}

	// Add all additional fields
	if p.AdditionalFields != nil {
		for k, v := range p.AdditionalFields {
			// Ensure we don't override the _meta field if it was already set from p.Meta
			// This check is important if AdditionalFields could also contain a "_meta" key,
			// though generally, _meta should be handled by the dedicated Meta field.
			if k != "_meta" {
				m[k] = v
			} else if _, metaExists := m["_meta"]; !metaExists {
				// If _meta was not set from p.Meta but exists in AdditionalFields, use it.
				// This case might be rare if p.Meta is the designated place for _meta.
				m[k] = v
			}
		}
	}
	if len(m) == 0 {
		// Return JSON representation of an empty object {} instead of null for empty params
		return []byte("{}"), nil
	}
	return json.Marshal(m)
}

// UnmarshalJSON implements custom JSON unmarshaling for NotificationParams.
// It separates '_meta' from other fields which are placed into AdditionalFields.
func (p *NotificationParams) UnmarshalJSON(data []byte) error {
	// Handle null or empty JSON object correctly for params
	sData := string(data)
	if sData == "null" || sData == "{}" {
		// If params is null or an empty object, initialize and return
		p.AdditionalFields = make(map[string]interface{})
		p.Meta = make(Meta) // Initialize Meta as well
		return nil
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}

	if p.AdditionalFields == nil {
		p.AdditionalFields = make(map[string]interface{})
	}
	// Ensure Meta is initialized if it's going to be populated or checked
	// p.Meta might be nil initially.
	// if p.Meta == nil { // Not strictly needed here as we assign directly or check m["_meta"]
	// 	p.Meta = make(Meta)
	// }

	for k, v := range m {
		if k == "_meta" {
			if metaMap, ok := v.(map[string]interface{}); ok {
				// Initialize p.Meta only if it's nil and metaMap is not nil and not empty
				if p.Meta == nil && metaMap != nil && len(metaMap) > 0 {
					p.Meta = make(Meta)
				}
				// Populate p.Meta. This handles case where p.Meta was nil or already existed.
				if p.Meta != nil { // ensure p.Meta is not nil before assigning to it
					for mk, mv := range metaMap {
						p.Meta[mk] = mv
					}
				}
			}
			// else: you might want to handle cases where _meta is not a map[string]interface{}
			// or log a warning, depending on strictness.
		} else {
			p.AdditionalFields[k] = v
		}
	}
	return nil
}

// Result is the base result struct for all MCP results.
type Result struct {
	Meta map[string]interface{} `json:"_meta,omitempty"`
}

// PaginatedResult is the base paginated result struct for all MCP paginated results.
type PaginatedResult struct {
	Result
	NextCursor Cursor `json:"nextCursor,omitempty"`
}

// ProgressToken is the base progress token struct for all MCP progress tokens.
type ProgressToken interface{}

// Cursor is the base cursor struct for all MCP cursors.
type Cursor string
