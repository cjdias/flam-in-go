package secrets

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.secrets"

// Configuration paths and container groups of the stores.
const (
	// StoreConfigPath is the configuration path holding one store
	// configuration per store id, as read by the store factory.
	StoreConfigPath = "flam.secrets.stores"

	// StoreCreatorGroup is the container group the store driver bundles
	// provide their [flam.ISecretStoreCreator] in, collected by the store
	// factory.
	StoreCreatorGroup = "flam.secrets.stores.creator"
)

// Context fields of the [*flam.Error] values returned by the store
// drivers.
const (
	// ErrorFieldID holds the id of a store.
	ErrorFieldID = "id"

	// ErrorFieldKey holds the key of a secret, or the path given to
	// GetMap, never its value.
	ErrorFieldKey = "key"
)
