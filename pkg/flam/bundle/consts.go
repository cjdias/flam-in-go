package bundle

const (
	// ErrorFieldIndex holds the position, from 0, of the queued
	// entry causing the error.
	ErrorFieldIndex = "index"

	// ErrorFieldKind holds the kind of the queued entry causing
	// the error: "bundle", "provider", "invoker" or "decorator".
	ErrorFieldKind = "kind"
)
