// Package awssm is the secret store driver of AWS Secrets Manager: its
// stores read the secrets of the service with the AWS SDK for Go v2, as a
// [github.com/cjdias/flam-in-go/pkg/flam.ISecretStore].
//
// A key is the name, or the ARN, of a secret, such as "prod/db", whose
// whole value is returned, or of the name#field form, such as
// "prod/db#password", naming the field password of the JSON object held
// by the secret. GetMap returns every field of the JSON object of the
// secret of its name. A field holding a string is returned as is, and any
// other JSON value, such as a number or an object, as its JSON text. The
// value of a secret is its SecretString or, for a binary secret, its
// SecretBinary, of the version labelled with the version stage of the
// store.
//
// A store is configured with:
//   - [ConfigFieldRegion]: the region of the service, such as
//     "eu-central-1", by default the one of the AWS_REGION environment
//     variable or of the shared configuration;
//   - [ConfigFieldEndpoint]: the URL of the service, replacing the one of
//     the region, such as "http://localhost:4566" for a LocalStack;
//   - [ConfigFieldProfile]: the profile of the shared configuration;
//   - [ConfigFieldAccessKeyID], [ConfigFieldSecretAccessKey] and
//     [ConfigFieldSessionToken]: static credentials, by default the
//     credentials of the default chain of the SDK, such as those of the
//     environment, of the shared configuration, or of the role of the
//     instance or task;
//   - [ConfigFieldVersionStage]: the staging label of the versions read,
//     "AWSCURRENT" by default;
//   - [ConfigFieldTimeout]: the bound of each read, retries included, 10s
//     by default, zero leaving the context as the only bound, and
//     [ConfigFieldMaxAttempts], the maximum number of attempts of a
//     request, 3 by default;
//   - [ConfigFieldCacheTTL]: how long the value of a secret is kept, read
//     from the [github.com/cjdias/flam-in-go/pkg/flam.IClock] of the
//     container, 30s by default, zero disabling the cache.
//
// A missing field takes the value stored at its default path, such as
// [DefaultRegionConfigPath]. The configuration of the SDK is loaded when
// the store is created, the credentials being resolved on the first read.
// A ResourceNotFoundException is a secret not found. The errors never hold
// the credentials, nor a secret value: the error of the decoding of a
// value that is not a JSON object is dropped, as it may quote the value.
package awssm
