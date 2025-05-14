package mcp

import "encoding/json"

// ClientRequest represents all client requests.
type ClientRequest interface{}

// ClientNotification represents all client notifications.
type ClientNotification interface{}

// ClientResult represents all client results.
type ClientResult interface{}

// ServerRequest represents all server requests.
type ServerRequest interface{}

// ServerNotification represents all server notifications.
type ServerNotification interface{}

// ServerResult represents all server results.
type ServerResult interface{}

// LoggingLevel represents the severity of a log message
//
// These map to syslog message severities as defined in RFC-5424:
// https://datatracker.ietf.org/doc/html/rfc5424#section-6.2.1
type LoggingLevel string

// Logging level constants
const (
	LoggingLevelDebug     LoggingLevel = "debug"
	LoggingLevelInfo      LoggingLevel = "info"
	LoggingLevelNotice    LoggingLevel = "notice"
	LoggingLevelWarning   LoggingLevel = "warning"
	LoggingLevelError     LoggingLevel = "error"
	LoggingLevelCritical  LoggingLevel = "critical"
	LoggingLevelAlert     LoggingLevel = "alert"
	LoggingLevelEmergency LoggingLevel = "emergency"
)

// LoggingMessageParams defines parameters for a log message notification
type LoggingMessageParams struct {
	// Severity of this log message
	Level LoggingLevel `json:"level"`

	// Data to log, such as a string message or object
	// Any JSON-serializable type is allowed here
	Data interface{} `json:"data"`

	// Optional name of the logger that emitted this message
	Logger string `json:"logger,omitempty"`
}

// LoggingMessageNotification log message notification from server to client
type LoggingMessageNotification struct {
	Method string               `json:"method"`
	Params LoggingMessageParams `json:"params"`
}

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

// Role represents the sender or recipient of a message
type Role string

const (
	// RoleUser represents the user role
	RoleUser Role = "user"

	// RoleAssistant represents the assistant role
	RoleAssistant Role = "assistant"
)

// Annotated describes an annotated resource.
type Annotated struct {
	// Annotations (optional)
	Annotations *struct {
		Audience []Role  `json:"audience,omitempty"`
		Priority float64 `json:"priority,omitempty"`
	} `json:"annotations,omitempty"`
}

type Content interface {
	isContent()
}

// TextContent represents text content
type TextContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
	Annotated
}

func (TextContent) isContent() {}

// ImageContent represents image content
type ImageContent struct {
	Type     string `json:"type"`
	Data     string `json:"data"` // base64 encoded image data
	MimeType string `json:"mimeType"`
	Annotated
}

func (ImageContent) isContent() {}

// AudioContent represents audio content
type AudioContent struct {
	Type     string `json:"type"`
	Data     string `json:"data"` // base64 encoded audio data
	MimeType string `json:"mimeType"`
	Annotated
}

func (AudioContent) isContent() {}

// EmbeddedResource represents an embedded resource
type EmbeddedResource struct {
	Resource ResourceContents `json:"resource"` // Using generic interface type
	Type     string           `json:"type"`
	Annotated
}

func (EmbeddedResource) isContent() {}

// NewTextContent helpe functions for content creation
func NewTextContent(text string) TextContent {
	return TextContent{
		Type: "text",
		Text: text,
	}
}

func NewImageContent(data string, mimeType string) ImageContent {
	return ImageContent{
		Type:     "image",
		Data:     data,
		MimeType: mimeType,
	}
}

func NewAudioContent(data string, mimeType string) AudioContent {
	return AudioContent{
		Type:     "audio",
		Data:     data,
		MimeType: mimeType,
	}
}

func NewEmbeddedResource(resource ResourceContents) EmbeddedResource {
	return EmbeddedResource{
		Type:     "resource",
		Resource: resource,
	}
}
