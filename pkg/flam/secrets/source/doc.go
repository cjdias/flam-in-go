// Package source is the configuration source driver of the secrets: its
// sources read secrets from a
// [github.com/cjdias/flam-in-go/pkg/flam.ISecretStore] into configuration
// paths, as a [github.com/cjdias/flam-in-go/pkg/flam.IConfigSource], so
// that the configuration of an application can hold secrets without
// writing them in its files, such as:
//
//	flam:
//	  secrets:
//	    source:
//	      store: vault
//	      values:
//	        flam.database.connections.main.password: db#password
//
// A source is configured with:
//   - [ConfigFieldValues]: a bag mapping configuration paths to the keys of
//     their secrets. As the configuration expands the dotted keys of its
//     bags, the paths may be written whole, as above, or as nested bags:
//     each string held by the bag, at any depth, is the key of the secret
//     of its path;
//   - [ConfigFieldStore]: the id of the store of the secrets, in the
//     [github.com/cjdias/flam-in-go/pkg/flam.ISecretStoreFactory], required
//     when there are values;
//   - [ConfigFieldPriority]: the priority of the source;
//   - [ConfigFieldReload]: whether the source is reloaded, false by
//     default, as each reload reads every secret of the source again. When
//     false, the source is created as a
//     [github.com/cjdias/flam-in-go/pkg/flam.IConfigSource] only, not
//     implementing
//     [github.com/cjdias/flam-in-go/pkg/flam.IConfigReloadableSource], so
//     that the reloader of the configuration skips it.
//
// A missing field takes the value stored at [DefaultValuesConfigPath],
// [DefaultStoreConfigPath], [DefaultPriorityConfigPath] or
// [DefaultReloadConfigPath], set by the bundle to no value, no store, 0 and
// false. The bundle also configures a source of id [Id] with this driver,
// which the fields of the example above, at these default paths, describe.
//
// Every secret is read when the source is created, which fails when one of
// them cannot be read, and again on each reload, which keeps the previous
// values when one of them fails. The values are the strings of the
// secrets.
//
// # Order
//
// The configuration bundle creates its sources when the application boots,
// in id order, each from the configuration holding the sources added
// before it. The store factory is taken from the container when the first
// source holding values is created, and reads the configuration of its
// stores once, when created: the stores, and the fields of a source, can
// be configured by the sources whose id sorts before the id of the source,
// such as a file source of id "app" for the source of id [Id].
package source
