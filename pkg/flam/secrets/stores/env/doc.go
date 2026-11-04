// Package env is the secret store driver of the environment variables: its
// stores read each secret from the variable named after its key, as a
// [github.com/cjdias/flam-in-go/pkg/flam.ISecretStore].
//
// A key is turned into a variable name in upper snake case, prefixed with
// the [ConfigFieldPrefix] of the store: its letters are upper cased, an
// underscore is inserted between a lower case letter or a digit and the
// upper case letter following it, and any other character, such as the #
// of the path#field form, a dot or a slash, becomes an underscore. With the
// "APP_" prefix, the keys "db#password", "db.password" and "dbPassword" all
// read APP_DB_PASSWORD.
//
// GetMap reads the variables whose name starts with the name of the path
// followed by an underscore, or with the prefix alone for an empty path,
// each under the rest of its name in lower case: with the "APP_" prefix,
// GetMap of "db" returns APP_DB_USER and APP_DB_PASSWORD as "user" and
// "password".
//
// A variable that is not set is not found, while a variable set to the
// empty string is an empty secret. The process environment is read on each
// call, and never changed.
package env
