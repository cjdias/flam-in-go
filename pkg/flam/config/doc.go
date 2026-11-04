// Package config is the configuration feature: the bundle returned from
// [NewBundle] provides the configuration of an application, the
// implementation of the [github.com/cjdias/flam-in-go/pkg/flam.IConfig]
// contract, and the
// [github.com/cjdias/flam-in-go/pkg/flam.IConfigParserFactory] and
// [github.com/cjdias/flam-in-go/pkg/flam.IConfigSourceFactory], which
// create the parsers and sources described by the configuration at
// [ParserConfigPath] and [SourceConfigPath] with the creators provided in
// the [ParserCreatorGroup] and [SourceCreatorGroup] container groups.
//
// The configuration starts empty: the application adds to it its initial
// values and the defaults of its bundles. When the application boots, the
// bundle adds the sources configured at
// [SourceConfigPath] to that configuration, unless [BootConfigPath] holds
// false, and reloads them at the period stored at [ReloadPeriodConfigPath].
//
// # Configuration
//
// The configuration merges its layers, the defaults, the sources and the
// values stored with Set, as described by
// [github.com/cjdias/flam-in-go/pkg/flam.IConfig], again on each change,
// into a new bag replacing the previous one. A layer is merged as by
// [github.com/cjdias/flam-in-go/pkg/flam/container.Bag.Merge], so that a
// key holding dots, such as "a.b", is expanded into nested values: a layer
// holding a key that is not a valid path, such as "a..b", is refused.
//
// The drivers live in the sub packages of
// [github.com/cjdias/flam-in-go/pkg/flam/config/parsers] and
// [github.com/cjdias/flam-in-go/pkg/flam/config/sources]. They import this
// package for its paths and groups, so this package never imports them.
package config
