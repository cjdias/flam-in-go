// Package s3 is the disk driver of the disks over a bucket of Amazon S3,
// or of a S3 compatible storage, such as MinIO or localstack, with the
// [github.com/aws/aws-sdk-go-v2/service/s3] client: the files of a disk are
// the objects of the bucket whose keys have the prefix of the disk, its
// root, as described by
// [github.com/cjdias/flam-in-go/pkg/flam/disk/disks/internal/objectfs].
//
// A disk reaches the bucket of its [ConfigFieldBucket] field in the region
// of its [ConfigFieldRegion] field, through the endpoint of its
// [ConfigFieldEndpoint] field, if any, addressing the bucket in the path
// of the requests when its [ConfigFieldPathStyle] field is true. It
// authenticates with the static credentials of its [ConfigFieldAccessKey],
// [ConfigFieldSecretKey] and [ConfigFieldSessionToken] fields, or else
// with those of the default chain of AWS. The other settings of the
// client, such as the retries, are taken from the AWS environment and
// shared configuration, as by
// [github.com/aws/aws-sdk-go-v2/config.LoadDefaultConfig].
//
// The disks hold nothing to release: closing them, as the factory does,
// does nothing.
package s3
