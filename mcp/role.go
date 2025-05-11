package mcp

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
