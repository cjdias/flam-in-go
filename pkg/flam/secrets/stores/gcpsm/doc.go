// Package gcpsm is the secret store driver of Google Cloud Secret Manager:
// its stores read the secrets of a project with the gRPC client of the
// service, as a [github.com/cjdias/flam-in-go/pkg/flam.ISecretStore].
//
// A key is the id of a secret, such as "db-credentials", whose whole value
// is returned, or of the id#field form, such as "db-credentials#password",
// naming the field password of the JSON object held by the secret, read
// from the version
//
//	projects/{project}/secrets/{id}/versions/{version}
//
// An id is made of 1 to 255 letters, digits, hyphens and underscores, as
// the service requires. GetMap returns every field of the JSON object of
// the secret of its id. A field holding a string is returned as is, and
// any other JSON value, such as a number or an object, as its JSON text.
// A payload whose CRC32C checksum does not match fails with [ErrResponse].
//
// A store is configured with:
//   - [ConfigFieldProject]: the required id, or number, of the project;
//   - [ConfigFieldVersion]: the version read, "latest" by default, or a
//     number or an alias;
//   - [ConfigFieldEndpoint]: the host:port of the service, such as
//     "secretmanager.europe-west1.rep.googleapis.com:443" for a regional
//     endpoint, by default the global one;
//   - [ConfigFieldCredentialsFile], or else [ConfigFieldCredentialsJSON],
//     of the type of [ConfigFieldCredentialsType], "service_account" by
//     default: the credentials of the client, by default the Application
//     Default Credentials, such as those of GOOGLE_APPLICATION_CREDENTIALS
//     or of the metadata server;
//   - [ConfigFieldInsecure]: whether the service of the endpoint is reached
//     without TLS nor authentication, for an emulator or tests only;
//   - [ConfigFieldTimeout]: the bound of each read, retries included, 10s
//     by default, zero leaving the context as the only bound;
//   - [ConfigFieldCacheTTL]: how long the value of a secret is kept, read
//     from the [github.com/cjdias/flam-in-go/pkg/flam.IClock] of the
//     container, 30s by default, zero disabling the cache.
//
// A missing field takes the value stored at its default path, such as
// [DefaultProjectConfigPath]. The credentials are loaded when the store is
// created, and closing the store closes its client. A NotFound status is a
// secret not found. The errors never hold the credentials, nor a secret
// value: the error of the loading of the credentials JSON, and of the
// decoding of a value that is not a JSON object, are dropped, as they may
// quote them.
package gcpsm
