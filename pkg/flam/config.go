package flam

import (
	"io"
	"net/http"
	"time"
)

// IConfigParser decodes configuration content, such as a JSON or YAML
// document, into a bag, for the sources reading files or HTTP responses.
// Parsers are created by the [IConfigParserFactory] from the
// configuration at
// [github.com/cjdias/flam-in-go/pkg/flam/config.ParserConfigPath], and
// closed by it when removed. A parser is safe for concurrent use.
type IConfigParser interface {
	io.Closer
	IResource

	// Parse decodes the whole content of the reader into a new bag. It
	// does not close the reader.
	Parse(reader io.Reader) (IBag, error)
}

// IConfigParserCreator creates the parsers of a format, for the
// [IConfigParserFactory].
type IConfigParserCreator = IResourceCreator[IConfigParser]

// IConfigParserFactory creates and holds the parsers described by the
// configuration at
// [github.com/cjdias/flam-in-go/pkg/flam/config.ParserConfigPath], with the
// creators of the parser formats.
type IConfigParserFactory = IFactory[IConfigParser]

// IConfigSource is a layer of configuration values, such as the content
// of a file, of environment variables or of an HTTP endpoint, added to an
// [IConfig] with AddSource. Sources are created by the
// [IConfigSourceFactory] from the configuration at
// [github.com/cjdias/flam-in-go/pkg/flam/config.SourceConfigPath], and
// closed by it when removed. A source is safe for concurrent use.
type IConfigSource interface {
	io.Closer
	IResource

	// Priority returns the priority of the source, which does not change:
	// the values of a source override those of the sources of lower
	// priority, as described by [IConfig].
	Priority() int

	// Bag returns a copy of the values of the source.
	Bag() IBag
}

// IConfigReloadableSource is an [IConfigSource] whose values may change,
// such as a file being edited, reloaded periodically by the configuration
// bundle, as described by
// [github.com/cjdias/flam-in-go/pkg/flam/config.ReloadPeriodConfigPath].
type IConfigReloadableSource interface {
	IConfigSource

	// Reload reads the values of the source again, and reports whether
	// they changed. A failed Reload keeps the previous values.
	Reload() (bool, error)
}

// IConfigSourceCreator creates the sources of a kind, for the
// [IConfigSourceFactory].
type IConfigSourceCreator = IResourceCreator[IConfigSource]

// IConfigSourceFactory creates and holds the sources described by the
// configuration at
// [github.com/cjdias/flam-in-go/pkg/flam/config.SourceConfigPath], with the
// creators of the source kinds.
type IConfigSourceFactory = IFactory[IConfigSource]

// IConfigRestClient sends the HTTP requests of the sources reading their
// values from a REST endpoint. An [*http.Client] is one.
type IConfigRestClient interface {
	// Do sends the request and returns its response, as
	// [http.Client.Do].
	Do(req *http.Request) (*http.Response, error)
}

// IConfigRestClientCreator creates the [IConfigRestClient] of a REST
// source. Provided by the application container, it lets an application
// configure the HTTP client, and a test replace it.
type IConfigRestClientCreator interface {
	// Create returns a new client.
	Create() (IConfigRestClient, error)
}

// ConfigObserver is a function observing the changes of a configuration
// path, registered with [IConfig.AddObserver].
//
// It is called with the values stored at the observed path before and
// after a change, nil standing for a missing value. Both are copies the
// observer may keep or change.
type ConfigObserver func(old, newValue any)

// IConfig is the configuration of an application, shared by its bundles
// and observable by path.
//
// The configuration is a bag of values addressed by dot separated paths,
// read with getters behaving as the [IBag] methods of the same name, which
// [github.com/cjdias/flam-in-go/pkg/flam/container.Bag]
// describes in full. Unlike the bag ones, the getters returning a map or a
// slice return a copy, as the configuration is shared: changing a returned
// value never changes the configuration.
//
// # Layers
//
// The values are those of layers merged, as by
// [github.com/cjdias/flam-in-go/pkg/flam/container.Bag.Merge], from the
// lowest to the highest:
//   - the defaults, added with AddDefaults, such as by the application
//     with those of the bundles implementing [IConfigurableBundle];
//   - the sources, added with AddSource, by increasing priority and, for
//     equal priorities, by id;
//   - the values stored with Set, which therefore override every source and
//     are kept when the sources change.
//
// Adding, removing or reloading a source, with AddSource, RemoveSource and
// ReloadSources, merges the layers again, so that the values of a removed
// source disappear unless another layer holds them. The configuration
// provided by the configuration bundle starts with the defaults only: as
// the parsers and sources are themselves created from the configuration,
// the sources are added afterwards, when the application boots, as
// described by
// [github.com/cjdias/flam-in-go/pkg/flam/config.BootConfigPath].
//
// # Observers
//
// Observers watch a path, and are identified by an id unique among all the
// observers, whatever their path. After each change of the value stored
// at an observed path, by Set, whether it targets that path, one under it
// or one above it, or by a change of the defaults or the sources, the
// callbacks of its observers are called, in observer id order, with the old
// and new values.
// A value changes when the old and new ones are not equal as
// [reflect.DeepEqual] tells, but for the NaN floats, equal to each other,
// and the nil and empty bags and lists, equal too, so storing an equal
// value calls no observer.
//
// The callbacks are called by the goroutine making the change, after it
// and once the configuration is released, so they may read or Set it
// themselves. A callback calling Set triggers the observers of that change
// before its own Set returns: an observer setting the path it watches to a
// new value on each call never ends. When several goroutines change the
// configuration at the same time, the order in which their observers are
// called is unspecified.
//
// The errors returned are [*Error] values wrapping one of the sentinel
// errors of [github.com/cjdias/flam-in-go/pkg/flam/config],
// or of the bag for the paths.
//
// An IConfig is safe for concurrent use.
type IConfig interface {
	// Entries returns, sorted, the keys of the top level values.
	Entries() []string

	// Has reports whether a value is stored at the path. See
	// [github.com/cjdias/flam-in-go/pkg/flam/container.Bag.Has].
	Has(path string) bool

	// Get returns a copy of the value stored at the path, or the first
	// default. See [github.com/cjdias/flam-in-go/pkg/flam/container.Bag.Get].
	Get(path string, def ...any) any

	// Bool returns the bool stored at the path, or the first default.
	Bool(path string, def ...bool) bool

	// Int returns the int stored at the path, or the first default.
	Int(path string, def ...int) int

	// Int8 returns the int8 stored at the path, or the first default.
	Int8(path string, def ...int8) int8

	// Int16 returns the int16 stored at the path, or the first default.
	Int16(path string, def ...int16) int16

	// Int32 returns the int32 stored at the path, or the first default.
	Int32(path string, def ...int32) int32

	// Int64 returns the int64 stored at the path, or the first default.
	Int64(path string, def ...int64) int64

	// Uint returns the uint stored at the path, or the first default.
	Uint(path string, def ...uint) uint

	// Uint8 returns the uint8 stored at the path, or the first default.
	Uint8(path string, def ...uint8) uint8

	// Uint16 returns the uint16 stored at the path, or the first default.
	Uint16(path string, def ...uint16) uint16

	// Uint32 returns the uint32 stored at the path, or the first default.
	Uint32(path string, def ...uint32) uint32

	// Uint64 returns the uint64 stored at the path, or the first default.
	Uint64(path string, def ...uint64) uint64

	// Float32 returns the float32 stored at the path, or the first default.
	Float32(path string, def ...float32) float32

	// Float64 returns the float64 stored at the path, or the first default.
	Float64(path string, def ...float64) float64

	// Rune returns the rune stored at the path, or the first default.
	Rune(path string, def ...rune) rune

	// String returns the string stored at the path, or the first default.
	String(path string, def ...string) string

	// StringMap returns a copy of the bag, a Bag or a map[string]any,
	// stored at the path as a map[string]any, or the first default.
	StringMap(path string, def ...map[string]any) map[string]any

	// StringMapString returns the bag stored at the path as a new
	// map[string]string, or the first default when no bag is stored there
	// or one of its values is not a string. A stored map[string]string is
	// returned as a copy.
	StringMapString(path string, def ...map[string]string) map[string]string

	// Slice returns a copy of the []any stored at the path, or the first
	// default. See [github.com/cjdias/flam-in-go/pkg/flam/container.Bag.Slice].
	Slice(path string, def ...[]any) []any

	// StringSlice returns a copy of the []string stored at the path, or the
	// first default. See
	// [github.com/cjdias/flam-in-go/pkg/flam/container.Bag.StringSlice].
	StringSlice(path string, def ...[]string) []string

	// Duration returns the time.Duration stored at the path, or the string
	// stored there parsed with [time.ParseDuration], such as "5s", as read
	// from a configuration file, or the first default when there is neither
	// or the string does not parse. Unlike [IBag.Duration], it does parse
	// strings; integers are still not taken as nanoseconds.
	Duration(path string, def ...time.Duration) time.Duration

	// Bag returns a copy of the bag stored at the path, or the first
	// default. See [github.com/cjdias/flam-in-go/pkg/flam/container.Bag.Bag].
	Bag(path string, def ...IBag) IBag

	// Set stores a copy of the value at the path of the Set layer, as
	// [github.com/cjdias/flam-in-go/pkg/flam/container.Bag.Set] does, and
	// then calls the observers whose value changed, as described above. It
	// returns the errors of the bag Set.
	Set(path string, value any) error

	// Populate decodes the whole configuration into target, which must be
	// a non-nil pointer, through encoding/json: struct fields are matched by
	// their json tag or, case insensitively, by their name. The infinite
	// and NaN floats, which JSON cannot hold, are decoded as null, leaving
	// the field they would set unchanged. It fails with
	// config.ErrInvalidTarget, or the json error.
	Populate(target any) error

	// PopulatePath decodes the value stored at the path into target, as
	// Populate does. It fails with config.ErrInvalidTarget,
	// config.ErrMissingPath when no value is stored at the path, or the json
	// error.
	PopulatePath(path string, target any) error

	// AddDefaults merges a copy of the values into the defaults layer,
	// where it holds no value, so that the defaults added first win, then
	// calls the observers whose value changed. It fails with the error of
	// the bag merge, such as for a key that is not a valid path, leaving
	// the configuration unchanged.
	AddDefaults(defaults IBag) error

	// HasSource reports whether a source with the id was added.
	HasSource(id string) bool

	// AddSource adds the source as a layer, then calls the observers whose
	// value changed. It fails with config.ErrNilSource, or
	// config.ErrDuplicateSource when a source with the same id was added.
	AddSource(source IConfigSource) error

	// RemoveSource removes the source with the id, without closing it,
	// then calls the observers whose value changed. It fails with
	// config.ErrUnknownSource when no source has the id.
	RemoveSource(id string) error

	// ReloadSources reloads the added sources implementing
	// [IConfigReloadableSource], one after the other, merges the layers
	// again when one changed, then calls the observers whose value
	// changed. A source failing to reload, or whose new values cannot be
	// merged, keeps its previous values; the errors of the failed sources,
	// each an [*Error] holding the source id, are returned joined, with
	// [errors.Join]. A source removed while reloaded, then added again,
	// keeps the values reloaded when the source added again is the same,
	// comparable, value, and the values it is added again with otherwise.
	// As the observers are called once the reload is over, they may call
	// ReloadSources themselves.
	ReloadSources() error

	// HasObserver reports whether the observer identified by id exists
	// and watches the path.
	HasObserver(id, path string) bool

	// AddObserver adds an observer, identified by id, calling callback
	// when the value stored at the path changes. It fails with
	// config.ErrNilObserver, config.ErrDuplicateObserver when an observer
	// already uses the id, whatever its path, or, for a malformed path,
	// [github.com/cjdias/flam-in-go/pkg/flam/container.ErrInvalidPath].
	AddObserver(id, path string, callback ConfigObserver) error

	// RemoveObserver removes the observer identified by id. It fails with
	// config.ErrUnknownObserver when no observer uses the id.
	RemoveObserver(id string) error
}
