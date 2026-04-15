package tool

// CapabilityDeclarer is an optional interface for tools that self-declare
// their required capabilities. Checked via type assertion on tool.Tool.
//
//	if cd, ok := t.(CapabilityDeclarer); ok {
//	    caps := cd.RequiredCapabilities()
//	}
type CapabilityDeclarer interface {
	RequiredCapabilities() []string
}

// ToolMetadata is an optional interface that tools can implement to
// advertise extended metadata. Consumers check via type assertion:
//
//	if tm, ok := t.(ToolMetadata); ok {
//	    meta := tm.Metadata()
//	}
type ToolMetadata interface {
	Metadata() Metadata
}

// Metadata holds extended tool metadata.
type Metadata struct {
	MaxResultSize int      `json:"max_result_size,omitempty"` // max output bytes; 0 = unlimited
	Capabilities  []string `json:"capabilities,omitempty"`    // required RBAC capabilities
}

// Availability is an optional interface for tools with dynamic availability.
// Tools that implement this and return false are excluded from All()/Names()
// and return ErrNotFound from Execute().
//
//	if a, ok := t.(Availability); ok && !a.Available() {
//	    // tool is currently unavailable
//	}
type Availability interface {
	Available() bool
}
