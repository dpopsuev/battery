package tool

// Content is a single content block in a tool result.
// Implementations: TextContent, ImageContent, AudioContent, ResourceLink, ResourceContent.
type Content interface {
	ContentType() string
}

// TextContent holds plain text output.
type TextContent struct {
	Text string `json:"text"`
}

// ContentType returns "text".
func (TextContent) ContentType() string { return "text" }

// ImageContent holds base64-encoded image data.
type ImageContent struct {
	MIMEType string `json:"mimeType"`
	Data     []byte `json:"data"`
}

// ContentType returns "image".
func (ImageContent) ContentType() string { return "image" }

// AudioContent holds base64-encoded audio data.
type AudioContent struct {
	MIMEType string `json:"mimeType"`
	Data     []byte `json:"data"`
}

// ContentType returns "audio".
func (AudioContent) ContentType() string { return "audio" }

// ResourceLink is a reference to an external resource.
type ResourceLink struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MIMEType    string `json:"mimeType,omitempty"`
}

// ContentType returns "resource_link".
func (ResourceLink) ContentType() string { return "resource_link" }

// ResourceContent holds embedded resource data.
type ResourceContent struct {
	URI      string `json:"uri"`
	MIMEType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
	Blob     []byte `json:"blob,omitempty"`
}

// ContentType returns "resource".
func (ResourceContent) ContentType() string { return "resource" }
