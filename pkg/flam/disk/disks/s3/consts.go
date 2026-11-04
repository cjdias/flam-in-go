package s3

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.disk.disks.s3"

// Driver is the driver name of the s3 disks, held by the
// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver] field
// of their configuration.
const Driver = "flam.disk.disks.driver.s3"

// Fields of a s3 disk configuration.
const (
	// ConfigFieldBucket is the field holding the name of the bucket of the
	// disk, which is required.
	ConfigFieldBucket = "bucket"

	// ConfigFieldPrefix is the field holding the root of the disk inside
	// the bucket, the prefix of the keys of its objects, such as
	// "uploads", the whole bucket unless set.
	ConfigFieldPrefix = "prefix"

	// ConfigFieldRegion is the field holding the region of the bucket,
	// such as "eu-west-1", an empty region being taken from the AWS
	// environment and shared configuration.
	ConfigFieldRegion = "region"

	// ConfigFieldEndpoint is the field holding the URL of an endpoint
	// replacing the one of AWS, such as "http://localhost:9000" for MinIO
	// or localstack, the one of AWS when empty.
	ConfigFieldEndpoint = "endpoint"

	// ConfigFieldPathStyle is the field holding whether the bucket is
	// addressed in the path of the requests, as most S3 compatible
	// storages require, rather than in the host name, a bool.
	ConfigFieldPathStyle = "path_style"

	// ConfigFieldAccessKey is the field holding the access key id of the
	// static credentials of the disk, set along with
	// [ConfigFieldSecretKey]. Without it, or when empty, the credentials
	// are taken from the default chain of AWS: the environment, the shared
	// files and the roles of the platform.
	ConfigFieldAccessKey = "access_key"

	// ConfigFieldSecretKey is the field holding the secret access key of
	// the static credentials of the disk, set along with
	// [ConfigFieldAccessKey].
	ConfigFieldSecretKey = "secret_key" //nolint:gosec // a field name, not a credential

	// ConfigFieldSessionToken is the field holding the session token of
	// temporary static credentials, set only along with
	// [ConfigFieldAccessKey].
	ConfigFieldSessionToken = "session_token" //nolint:gosec // a field name, not a credential
)

// Configuration paths of the values taken by the fields missing from a
// disk configuration.
const (
	// DefaultRegionConfigPath is the path of the default of
	// [ConfigFieldRegion], "us-east-1" unless changed.
	DefaultRegionConfigPath = "flam.disk.defaults.disks.s3.region"

	// DefaultEndpointConfigPath is the path of the default of
	// [ConfigFieldEndpoint], empty unless changed.
	DefaultEndpointConfigPath = "flam.disk.defaults.disks.s3.endpoint"

	// DefaultPathStyleConfigPath is the path of the default of
	// [ConfigFieldPathStyle], false unless changed.
	DefaultPathStyleConfigPath = "flam.disk.defaults.disks.s3.path_style"
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
	defaultRegion    = "us-east-1"
	defaultEndpoint  = ""
	defaultPathStyle = false
)

// diskName is the name of the s3 disks, returned by their Name method.
const diskName = "S3Fs"
