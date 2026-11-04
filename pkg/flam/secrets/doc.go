// Package secrets is the secrets feature: the bundle returned from
// [NewBundle] provides the
// [github.com/cjdias/flam-in-go/pkg/flam.ISecretStoreFactory] of an
// application, which creates the secret stores described by the
// configuration at [StoreConfigPath] with the creators provided in the
// [StoreCreatorGroup] container group, such as:
//
//	flam:
//	  secrets:
//	    stores:
//	      vault:
//	        driver: flam.secrets.stores.driver.vault
//	        address: https://vault.example.com:8200
//	        token_file: /var/run/vault/token
//
// A store reads a secret by its key, the path#field form, such as
// "db#password", naming the field of the secret at a path for every
// driver: the environment variable DB_PASSWORD for the env driver, the
// file db/password for the file driver, and the field password of the
// secret db for the vault driver.
//
// The errors of the stores wrap [ErrNotFound] for a secret that does not
// exist, and [ErrInvalidKey] for a key the driver refuses, whatever the
// driver, so that their callers can tell them apart. They never hold a
// secret value.
//
// The store drivers live in the sub packages of
// [github.com/cjdias/flam-in-go/pkg/flam/secrets/stores], while
// [github.com/cjdias/flam-in-go/pkg/flam/secrets/source] is the
// configuration source driver loading secrets into the configuration. They
// import this package for its paths, groups and errors, so this package
// never imports them.
package secrets
