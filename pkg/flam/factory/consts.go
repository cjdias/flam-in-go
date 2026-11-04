package factory

// Context fields of the [*flam.Error] values returned by the [flam.IFactory]
// methods.
const (
	// ErrorFieldID holds the resource id given to the method.
	ErrorFieldID = "id"

	// ErrorFieldResourceID holds the id returned by the Id method of
	// a resource whose id does not match, with [ErrIDMismatch].
	ErrorFieldResourceID = "resource_id"
)

// Fields of the resource configurations given to the [flam.IFactory]
// creators.
const (
	// ConfigFieldId holds the id of the resource to create, in the
	// configuration copy given to the [flam.IResourceCreator] methods, so
	// that the creator can give the resource that id. The factory sets it,
	// replacing any value of a field of that name in the configuration.
	ConfigFieldId = "id"

	// ConfigFieldDriver holds the name of the driver a resource
	// configuration asks for, which creators typically check in Accept.
	ConfigFieldDriver = "driver"
)
