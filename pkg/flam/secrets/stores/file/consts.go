package file

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.secrets.stores.file"

const (
	// Id is the id of the default store configured by the bundle returned
	// from [NewBundle].
	Id = "file"

	// Driver is the driver name of the file stores, held by the
	// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver]
	// field of their configuration.
	Driver = "flam.secrets.stores.driver.file"
)

// Fields of a file store configuration.
const (
	// ConfigFieldDir is the field holding the non-empty path of the
	// directory of the secret files.
	ConfigFieldDir = "dir"

	// ConfigFieldTrim is the field holding the bool telling whether the
	// trailing newline of a file, "\n" or "\r\n", is dropped from its
	// value.
	ConfigFieldTrim = "trim"
)

// Configuration paths of the values taken by the fields missing from a
// store configuration.
const (
	// DefaultDirConfigPath is the path of the default of [ConfigFieldDir],
	// "/run/secrets", where Docker mounts its secrets, unless changed.
	DefaultDirConfigPath = "flam.secrets.defaults.stores.file.dir"

	// DefaultTrimConfigPath is the path of the default of
	// [ConfigFieldTrim], true unless changed.
	DefaultTrimConfigPath = "flam.secrets.defaults.stores.file.trim"
)

// Context fields of the [*flam.Error] values returned by the creator and
// the stores, along with those of
// [github.com/cjdias/flam-in-go/pkg/flam/secrets].
const (
	// ErrorFieldID holds the id of the store.
	ErrorFieldID = "id"

	// ErrorFieldField holds the name of an invalid configuration field.
	ErrorFieldField = "field"

	// ErrorFieldKey holds the key of a secret, or the path given to
	// GetMap.
	ErrorFieldKey = "key"
)

// Defaults of the fields of a store configuration.
const (
	defaultDir  = "/run/secrets"
	defaultTrim = true
)
