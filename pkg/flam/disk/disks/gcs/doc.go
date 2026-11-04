// Package gcs is the disk driver of the disks over a bucket of Google
// Cloud Storage, or of an emulator of its JSON API, with the
// [cloud.google.com/go/storage] client: the files of a disk are the
// objects of the bucket whose names have the prefix of the disk, its
// root, as described by
// [github.com/cjdias/flam-in-go/pkg/flam/disk/disks/internal/objectfs].
//
// A disk reaches the bucket of its [ConfigFieldBucket] field through the
// JSON API, its objects being read with it too, at the endpoint of its
// [ConfigFieldEndpoint] field, if any. It authenticates with the
// credentials of its [ConfigFieldCredentialsFile] or
// [ConfigFieldCredentialsJSON] field, of the type of its
// [ConfigFieldCredentialsType] field, or sends no credentials when its
// [ConfigFieldAnonymous] field is true, or else authenticates with the
// application default credentials of Google Cloud.
//
// Each disk holds its own client, closed by the Close method of the disk,
// as the factory does.
package gcs
