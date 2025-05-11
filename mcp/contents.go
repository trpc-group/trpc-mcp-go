package mcp

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

// Helper functions for content creation
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
