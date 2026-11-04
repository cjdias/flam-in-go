// Package etcd is the configuration source driver of the etcd clusters:
// its sources read a key, or the tree of keys under a prefix, with the etcd
// v3 client, and decode the values with an
// [github.com/cjdias/flam-in-go/pkg/flam.IConfigParser], as a
// [github.com/cjdias/flam-in-go/pkg/flam.IConfigReloadableSource].
//
// A source is configured with:
//   - [ConfigFieldKey]: the key of the value, such as "/config/app.yaml",
//     or the prefix of the tree, required;
//   - [ConfigFieldRecursive]: whether the tree of the keys under the
//     prefix is read instead of the key alone, false by default;
//   - [ConfigFieldParser]: the id of the parser of the values, taken from
//     the [github.com/cjdias/flam-in-go/pkg/flam.IConfigParserFactory],
//     "json" by default, empty for the string values of a tree;
//   - [ConfigFieldEndpoints]: the endpoints of the cluster,
//     ["127.0.0.1:2379"] by default;
//   - [ConfigFieldUsername] and [ConfigFieldPassword]: the user of the
//     requests, authenticated when the source is created, none by default;
//   - [ConfigFieldTimeout]: the bound of each request, and of the
//     authentication, 10s by default, zero for no bound, a request to a
//     cluster that cannot be reached then waiting forever;
//   - [ConfigFieldCAFile]: the PEM file of the certificate authorities
//     trusted for the cluster, instead of those of the system,
//     [ConfigFieldCertFile] and [ConfigFieldKeyFile], the PEM files of a
//     client certificate and its key, and [ConfigFieldSkipVerify],
//     disabling the verification of the certificates of the cluster, for
//     tests only. The client talks TLS when one of them is set, or an
//     endpoint is an https URL;
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
// its default path, such as [DefaultEndpointsConfigPath]. Each source has
// a client of its own, closed with the source.
//
// # Values
//
// A source that is not recursive reads its key and parses its value as the
// document of the source, a missing key failing with [ErrMissingKey]. A
// recursive source reads every key starting with the prefix, taken as a
// folder, a slash being added when missing, and stores each value at the
// path of its key relative to the prefix, its slashes turned into dots:
// with the prefix "/config/app", the value of "/config/app/db/host" is
// stored at "db.host". With a parser, the value is parsed as a document and
// merged at that path; without, it is stored as its string. The keys
// ending with a slash are skipped, and an empty tree is an empty source. A
// key that cannot be stored, such as one holding an empty element, or one
// under a key holding a string, fails with [ErrTree].
//
// # Reload
//
// The key, or the tree, is read when the source is created, and again by
// Reload, called by the reloader of the configuration at its period. Keys
// whose count and highest modification revision are those of the values
// last loaded keep them, without parsing: every put raises the revision of
// the key written above any other, and a delete lowers the count. The
// source polls: it keeps no watch open, so that it holds no goroutine of
// its own and a Reload never waits for a change.
package etcd
