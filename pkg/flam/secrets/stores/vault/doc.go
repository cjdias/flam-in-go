// Package vault is the secret store driver of the HashiCorp Vault servers:
// its stores read the secrets of a KV version 2 secrets engine, over the
// HTTP API of Vault, as a
// [github.com/cjdias/flam-in-go/pkg/flam.ISecretStore]. It talks to Vault
// with [net/http] only, without the Vault client library.
//
// A key is of the path#field form, such as "db#password", naming the field
// password of the secret at the path db, read from
//
//	GET {address}/v1/{mount}/data/{path}
//
// with the token of the store in the X-Vault-Token header and, when set,
// the namespace of the store in the X-Vault-Namespace header. The path,
// and the mount, are slash separated, a key holding an empty, "." or ".."
// element being refused, and each of their elements is escaped. GetMap
// returns every field of the secret at its path. A field holding a string
// is returned as is, and any other JSON value, such as a number or an
// object, as its JSON text.
//
// A store is configured with:
//   - [ConfigFieldAddress]: the required URL of the server, such as
//     "https://vault.example.com:8200";
//   - [ConfigFieldMount]: the mount path of the secrets engine, "secret"
//     by default;
//   - [ConfigFieldNamespace]: the namespace of Vault Enterprise, none by
//     default;
//   - [ConfigFieldToken], or else [ConfigFieldTokenFile], or else the
//     VAULT_TOKEN environment variable, read when the store is created:
//     the token of the requests, the file being read again on each
//     request, so that an agent may renew it;
//   - [ConfigFieldTimeout]: the bound of each request, 10s by default,
//     zero leaving the context as the only bound;
//   - [ConfigFieldCAFile]: the PEM file of the certificate authorities
//     trusted for the server, instead of those of the system, and
//     [ConfigFieldSkipVerify], disabling the verification of the
//     certificate of the server, for tests only;
//   - [ConfigFieldCacheTTL]: how long the fields of a secret are kept,
//     read from the [github.com/cjdias/flam-in-go/pkg/flam.IClock] of the
//     container, 30s by default, zero disabling the cache.
//
// A missing field takes the value stored at its default path, such as
// [DefaultAddressConfigPath]. A response of status 404, or holding no
// data, as for a deleted version, is a secret not found. The errors never
// hold the token, nor a secret value.
package vault
