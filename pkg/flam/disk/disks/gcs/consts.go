package gcs

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.disk.disks.gcs"

// Driver is the driver name of the gcs disks, held by the
// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver] field
// of their configuration.
const Driver = "flam.disk.disks.driver.gcs"

// Fields of a gcs disk configuration.
const (
	// ConfigFieldBucket is the field holding the name of the bucket of the
	// disk, which is required.
	ConfigFieldBucket = "bucket"

	// ConfigFieldPrefix is the field holding the root of the disk inside
	// the bucket, the prefix of the names of its objects, such as
	// "uploads", the whole bucket unless set.
	ConfigFieldPrefix = "prefix"

	// ConfigFieldEndpoint is the field holding the URL of the JSON API
	// replacing the one of Google Cloud Storage, such as
	// "http://localhost:4443/storage/v1/" for an emulator, the one of
	// Google Cloud Storage when empty.
	ConfigFieldEndpoint = "endpoint"

	// ConfigFieldCredentialsFile is the field holding the path of the JSON
	// file of the credentials of the disk, of the type of
	// [ConfigFieldCredentialsType].
	ConfigFieldCredentialsFile = "credentials_file"

	// ConfigFieldCredentialsJSON is the field holding the JSON of the
	// credentials of the disk, of the type of
	// [ConfigFieldCredentialsType], set only without
	// [ConfigFieldCredentialsFile].
	ConfigFieldCredentialsJSON = "credentials_json"

	// ConfigFieldCredentialsType is the field holding the type of the
	// credentials of [ConfigFieldCredentialsFile] or
	// [ConfigFieldCredentialsJSON]: "service_account", "authorized_user",
	// "impersonated_service_account" or "external_account".
	ConfigFieldCredentialsType = "credentials_type"

	// ConfigFieldAnonymous is the field holding whether the disk sends its
	// requests without credentials, as for an emulator or a public bucket,
	// a bool, true only without [ConfigFieldCredentialsFile] and
	// [ConfigFieldCredentialsJSON]. Without credentials, the disk
	// authenticates with the application default credentials of Google
	// Cloud.
	ConfigFieldAnonymous = "anonymous"
)

// Configuration paths of the values taken by the fields missing from a
// disk configuration.
const (
	// DefaultEndpointConfigPath is the path of the default of
	// [ConfigFieldEndpoint], empty unless changed.
	DefaultEndpointConfigPath = "flam.disk.defaults.disks.gcs.endpoint"

	// DefaultCredentialsTypeConfigPath is the path of the default of
	// [ConfigFieldCredentialsType], "service_account" unless changed.
	DefaultCredentialsTypeConfigPath = "flam.disk.defaults.disks.gcs.credentials_type"

	// DefaultAnonymousConfigPath is the path of the default of
	// [ConfigFieldAnonymous], false unless changed.
	DefaultAnonymousConfigPath = "flam.disk.defaults.disks.gcs.anonymous"
)

// Context fields of the [*flam.Error] values returned by the creator.
const (
	// ErrorFieldID holds the id of the disk.
	ErrorFieldID = "id"

	// ErrorFieldField holds the name of an invalid configuration field.
	ErrorFieldField = "field"
)

// Defaults of the fields of a disk configuration.
const (
	defaultEndpoint        = ""
	defaultCredentialsType = "service_account"
	defaultAnonymous       = false
)

// diskName is the name of the gcs disks, returned by their Name method.
const diskName = "GCSFs"
