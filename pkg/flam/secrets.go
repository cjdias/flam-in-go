package flam

import (
	"context"
	"io"
)

// ISecretStore reads the secrets of an application, such as passwords or
// API tokens, from where they are kept: the environment, a directory of
// files mounted by Docker or Kubernetes, or a HashiCorp Vault server. It is
// held by an [ISecretStoreFactory], which creates it from the
// configuration at
// [github.com/cjdias/flam-in-go/pkg/flam/secrets.StoreConfigPath] and
// closes it when removed.
//
// A key names a secret, its syntax being the one of the driver, the
// path#field form, such as "db#password", naming the field of the secret
// at a path for every driver of flam. The context bounds the calls to the
// store, a nil one being replaced with [context.Background].
//
// The errors returned are [*Error] values wrapping, for a secret that does
// not exist,
// [github.com/cjdias/flam-in-go/pkg/flam/secrets.ErrNotFound], for a key
// the driver refuses,
// [github.com/cjdias/flam-in-go/pkg/flam/secrets.ErrInvalidKey], or else
// an error of the driver. They never hold a secret value.
//
// An ISecretStore is safe for concurrent use.
type ISecretStore interface {
	io.Closer
	IResource

	// Get returns the value of the secret of the key.
	Get(ctx context.Context, key string) (string, error)

	// GetMap returns the values of the secrets under the path, such as the
	// fields of a Vault secret or the files of a directory, by their key
	// relative to the path, in a new map the caller may keep.
	GetMap(ctx context.Context, path string) (map[string]string, error)
}

// ISecretStoreCreator creates the secret stores of a driver, for the
// [ISecretStoreFactory].
type ISecretStoreCreator = IResourceCreator[ISecretStore]

// ISecretStoreFactory creates and holds the secret stores described by the
// configuration at
// [github.com/cjdias/flam-in-go/pkg/flam/secrets.StoreConfigPath], with the
// creators of the store drivers.
type ISecretStoreFactory = IFactory[ISecretStore]
