package leader

// Context fields of the [*flam.Error] values returned by the leader
// package.
const (
	// ErrorFieldID holds the id of the process.
	ErrorFieldID = "id"

	// ErrorFieldKey holds the key the process campaigns for.
	ErrorFieldKey = "key"
)
