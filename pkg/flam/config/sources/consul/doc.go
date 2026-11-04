// Package consul is the configuration source driver of the HashiCorp Consul
// KV store: its sources read a key, or the tree of keys under a prefix,
// with the Consul client library, and decode the values with an
// [github.com/cjdias/flam-in-go/pkg/flam.IConfigParser], as a
// [github.com/cjdias/flam-in-go/pkg/flam.IConfigReloadableSource].
//
// A source is configured with:
//   - [ConfigFieldKey]: the key of the value, such as "config/app.json", or
//     the prefix of the tree, required;
//   - [ConfigFieldRecursive]: whether the tree of the keys under the
//     prefix is read instead of the key alone, false by default;
//   - [ConfigFieldParser]: the id of the parser of the values, taken from
//     the [github.com/cjdias/flam-in-go/pkg/flam.IConfigParserFactory],
//     "json" by default, empty for the string values of a tree;
//   - [ConfigFieldAddress]: the URL of the agent, "http://127.0.0.1:8500"
//     by default, a path, such as "https://gateway/consul", being the
//     prefix of the API behind a reverse proxy;
//   - [ConfigFieldToken]: the ACL token of the requests, and
//     [ConfigFieldDatacenter], the datacenter of the requests;
//   - [ConfigFieldTimeout]: the bound of each request, 10s by default, zero
//     for no bound;
//   - [ConfigFieldCAFile]: the PEM file of the certificate authorities
//     trusted for the agent, instead of those of the system,
//     [ConfigFieldCertFile] and [ConfigFieldKeyFile], the PEM files of a
//     client certificate and its key, and [ConfigFieldSkipVerify],
//     disabling the verification of the certificate of the agent, for tests
//     only;
//   - [ConfigFieldPriority]: the priority of the source;
//   - [ConfigFieldReload]: whether the source is reloaded, true by default.
//     When false, the source is created as a
//     [github.com/cjdias/flam-in-go/pkg/flam.IConfigSource] only, not
//     implementing
//     [github.com/cjdias/flam-in-go/pkg/flam.IConfigReloadableSource], so
//     that the reloader of the configuration skips it and its values never
//     change.
//
// A missing field, but the key and recursive, takes the value stored at
// its default path, such as [DefaultAddressConfigPath]. As with the Consul
// client, the empty token and files take the values of the CONSUL_HTTP_TOKEN,
// CONSUL_CACERT, CONSUL_CLIENT_CERT and CONSUL_CLIENT_KEY environment
// variables, when set.
//
// # Values
//
// A source that is not recursive reads its key with
//
//	GET {address}/v1/kv/{key}
//
// and parses its value as the document of the source, a missing key failing
// with [ErrMissingKey]. A recursive source reads every key under the
// prefix, taken as a folder, a slash being added when missing, with
//
//	GET {address}/v1/kv/{key}/?recurse
//
// and stores each value at the path of its key relative to the prefix, its
// slashes turned into dots: with the prefix "config/app", the value of
// "config/app/db/host" is stored at "db.host". With a parser, the value is
// parsed as a document and merged at that path; without, it is stored as
// its string. The folders, keys ending with a slash, are skipped, and an
// empty tree is an empty source. A key that cannot be stored, such as one
// holding an empty element, or one under a key holding a string, fails with
// [ErrTree].
//
// # Reload
//
// The key, or the tree, is read when the source is created, and again by
// Reload, called by the reloader of the configuration at its period. A
// response holding the X-Consul-Index of the values last loaded keeps them,
// without parsing: the agent changes the index of the KV store on every
// write. The source polls: it sends no blocking query, so that a Reload
// never waits for a change.
package consul
