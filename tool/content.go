// Package tool defines Battery-owned content types for tool results.
// Transport-agnostic — no MCP, HTTP, or protocol imports.
// Uses Strategy pattern: each content type registers its own codec.
// Adapters in mcp/ and mcpserver/ register transport-specific converters.
package tool

// Content is a single content block in a tool result.
type Content interface {
	contentType() string
}

// wireContent is the JSON wire format with a type discriminator.
type wireContent struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	MIMEType string `json:"mimeType,omitempty"`
	Data     []byte `json:"data,omitempty"`
	URI      string `json:"uri,omitempty"`
	Name     string `json:"name,omitempty"`
	Desc     string `json:"description,omitempty"`
	Blob     []byte `json:"blob,omitempty"`
}

// contentCodec handles serialization for a single content type.
type contentCodec struct {
	encode func(Content) wireContent
	decode func(wireContent) Content
}

// contentRegistry maps type discriminator to codec. Populated at init.
var contentRegistry = map[string]contentCodec{} //nolint:gochecknoglobals // strategy registry, populated at init

// RegisterContentType adds a content type codec. Called from init().
func RegisterContentType(typeName string, encode func(Content) wireContent, decode func(wireContent) Content) {
	contentRegistry[typeName] = contentCodec{encode: encode, decode: decode}
}

func encodeContent(c Content) (wireContent, bool) {
	codec, ok := contentRegistry[c.contentType()]
	if !ok {
		return wireContent{}, false
	}
	return codec.encode(c), true
}

func decodeContent(w wireContent) (Content, bool) {
	codec, ok := contentRegistry[w.Type]
	if !ok {
		return nil, false
	}
	return codec.decode(w), true
}

// TextContent holds plain text output.
type TextContent struct {
	Text string `json:"text"`
}

func (TextContent) contentType() string { return "text" }

// ImageContent holds base64-encoded image data.
type ImageContent struct {
	MIMEType string `json:"mimeType"`
	Data     []byte `json:"data"`
}

func (ImageContent) contentType() string { return "image" }

// AudioContent holds base64-encoded audio data.
type AudioContent struct {
	MIMEType string `json:"mimeType"`
	Data     []byte `json:"data"`
}

func (AudioContent) contentType() string { return "audio" }

// ResourceLink is a reference to an external resource.
type ResourceLink struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MIMEType    string `json:"mimeType,omitempty"`
}

func (ResourceLink) contentType() string { return "resource_link" }

// ResourceContent holds embedded resource data.
type ResourceContent struct {
	URI      string `json:"uri"`
	MIMEType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
	Blob     []byte `json:"blob,omitempty"`
}

func (ResourceContent) contentType() string { return "resource" }

func init() {
	RegisterContentType("text",
		func(c Content) wireContent {
			tc := c.(TextContent)
			return wireContent{Type: "text", Text: tc.Text}
		},
		func(w wireContent) Content {
			return TextContent{Text: w.Text}
		},
	)
	RegisterContentType("image",
		func(c Content) wireContent {
			ic := c.(ImageContent)
			return wireContent{Type: "image", MIMEType: ic.MIMEType, Data: ic.Data}
		},
		func(w wireContent) Content {
			return ImageContent{MIMEType: w.MIMEType, Data: w.Data}
		},
	)
	RegisterContentType("audio",
		func(c Content) wireContent {
			ac := c.(AudioContent)
			return wireContent{Type: "audio", MIMEType: ac.MIMEType, Data: ac.Data}
		},
		func(w wireContent) Content {
			return AudioContent{MIMEType: w.MIMEType, Data: w.Data}
		},
	)
	RegisterContentType("resource_link",
		func(c Content) wireContent {
			rl := c.(ResourceLink)
			return wireContent{Type: "resource_link", URI: rl.URI, Name: rl.Name, Desc: rl.Description, MIMEType: rl.MIMEType}
		},
		func(w wireContent) Content {
			return ResourceLink{URI: w.URI, Name: w.Name, Description: w.Desc, MIMEType: w.MIMEType}
		},
	)
	RegisterContentType("resource",
		func(c Content) wireContent {
			rc := c.(ResourceContent)
			return wireContent{Type: "resource", URI: rc.URI, MIMEType: rc.MIMEType, Text: rc.Text, Blob: rc.Blob}
		},
		func(w wireContent) Content {
			return ResourceContent{URI: w.URI, MIMEType: w.MIMEType, Text: w.Text, Blob: w.Blob}
		},
	)
}
