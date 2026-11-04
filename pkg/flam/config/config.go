package config

import (
	"cmp"
	"encoding/json"
	"errors"
	"maps"
	"math"
	"reflect"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/value"
)

// newConfig returns a new [flam.IConfig] holding no value, no source and
// no observer: the application adds the defaults of its bundles with
// AddDefaults, and the bundle adds the sources with AddSource. It is a
// provider registered by the bundle returned from [NewBundle].
//
// The configuration merges its layers again on each change, into a new bag
// replacing the previous one, which is never changed: the getters read the
// current bag, and copy the values they return, so that they never block
// for long and never see a change in progress.
func newConfig() flam.IConfig {
	return &configImpl{
		defaults:  container.Bag{},
		sources:   map[string]*configSource{},
		set:       container.Bag{},
		merged:    container.Bag{},
		observers: map[string]*configObserver{},
	}
}

// configImpl is the [flam.IConfig] returned by [newConfig].
type configImpl struct {
	mu        sync.RWMutex // guards the layers, merged and observers
	defaults  container.Bag
	sources   map[string]*configSource // replaced, never changed
	set       container.Bag            // replaced, never changed
	merged    container.Bag            // replaced, never changed
	observers map[string]*configObserver

	reloadMu sync.Mutex // serializes ReloadSources
}

// configSource is a source added to the configuration, with its priority
// and the values it held when added or last reloaded.
type configSource struct {
	id       string
	source   flam.IConfigSource
	priority int
	values   flam.IBag
}

// configReloaded holds the values reloaded by the source of an entry.
type configReloaded struct {
	entry  *configSource
	values flam.IBag
}

// configObserver is an observer of a configuration path.
type configObserver struct {
	path     string
	callback flam.ConfigObserver
}

// configCall is a call of an observer callback with the old and new values
// of its path, the values being read from bags never changed.
type configCall struct {
	callback flam.ConfigObserver
	old      any
	newValue any
}

// compile time check that *configImpl implements IConfig
var _ flam.IConfig = (*configImpl)(nil)

func (c *configImpl) Entries() []string {
	return slices.Sorted(maps.Keys(c.values()))
}

func (c *configImpl) Has(
	path string,
) bool {
	return c.values().Has(path)
}

func (c *configImpl) Get(
	path string,
	def ...any,
) any {
	return c.scoped(path).Get(configScopedKey, def...)
}

func (c *configImpl) Bool(
	path string,
	def ...bool,
) bool {
	return c.values().Bool(path, def...)
}

func (c *configImpl) Int(
	path string,
	def ...int,
) int {
	return c.values().Int(path, def...)
}

func (c *configImpl) Int8(
	path string,
	def ...int8,
) int8 {
	return c.values().Int8(path, def...)
}

func (c *configImpl) Int16(
	path string,
	def ...int16,
) int16 {
	return c.values().Int16(path, def...)
}

func (c *configImpl) Int32(
	path string,
	def ...int32,
) int32 {
	return c.values().Int32(path, def...)
}

func (c *configImpl) Int64(
	path string,
	def ...int64,
) int64 {
	return c.values().Int64(path, def...)
}

func (c *configImpl) Uint(
	path string,
	def ...uint,
) uint {
	return c.values().Uint(path, def...)
}

func (c *configImpl) Uint8(
	path string,
	def ...uint8,
) uint8 {
	return c.values().Uint8(path, def...)
}

func (c *configImpl) Uint16(
	path string,
	def ...uint16,
) uint16 {
	return c.values().Uint16(path, def...)
}

func (c *configImpl) Uint32(
	path string,
	def ...uint32,
) uint32 {
	return c.values().Uint32(path, def...)
}

func (c *configImpl) Uint64(
	path string,
	def ...uint64,
) uint64 {
	return c.values().Uint64(path, def...)
}

func (c *configImpl) Float32(
	path string,
	def ...float32,
) float32 {
	return c.values().Float32(path, def...)
}

func (c *configImpl) Float64(
	path string,
	def ...float64,
) float64 {
	return c.values().Float64(path, def...)
}

func (c *configImpl) Rune(
	path string,
	def ...rune,
) rune {
	return c.values().Rune(path, def...)
}

func (c *configImpl) String(
	path string,
	def ...string,
) string {
	return c.values().String(path, def...)
}

func (c *configImpl) StringMap(
	path string,
	def ...map[string]any,
) map[string]any {
	switch v := c.scoped(path).Get(configScopedKey).(type) {
	case container.Bag:
		return v
	case map[string]any:
		return v
	}
	return configFirst(def)
}

func (c *configImpl) StringMapString(
	path string,
	def ...map[string]string,
) map[string]string {
	switch v := c.scoped(path).Get(configScopedKey).(type) {
	case map[string]string:
		return v // a copy
	case container.Bag:
		if result, ok := configStrings(v); ok {
			return result
		}
	case map[string]any:
		if result, ok := configStrings(v); ok {
			return result
		}
	}
	return configFirst(def)
}

func (c *configImpl) Slice(
	path string,
	def ...[]any,
) []any {
	return c.scoped(path).Slice(configScopedKey, def...)
}

func (c *configImpl) StringSlice(
	path string,
	def ...[]string,
) []string {
	return c.scoped(path).StringSlice(configScopedKey, def...)
}

func (c *configImpl) Duration(
	path string,
	def ...time.Duration,
) time.Duration {
	switch v := c.values().Get(path).(type) {
	case time.Duration:
		return v
	case string:
		if duration, err := time.ParseDuration(v); err == nil {
			return duration
		}
	}
	return configFirst(def)
}

func (c *configImpl) Bag(
	path string,
	def ...flam.IBag,
) flam.IBag {
	return c.scoped(path).Bag(configScopedKey, def...)
}

func (c *configImpl) Set(
	path string,
	value any,
) error {
	return c.update(func() error {
		set := c.set.Clone().(container.Bag)
		if err := set.Set(path, configCopy(value)); err != nil {
			return err
		}
		// the layer is kept expanded, as the other layers once merged, so
		// that a key holding dots, such as "b.c" in a value set at "a",
		// is the path a later Set at "a.b.c" changes; a value holding a
		// key that is not a valid path is refused
		expanded, err := container.Bag{}.Merge("", set)
		if err != nil {
			return err
		}
		c.set = expanded.(container.Bag)
		c.remerge(c.defaults, c.sources)
		return nil
	})
}

func (c *configImpl) Populate(
	target any,
) error {
	if !configValidTarget(target) {
		return newErrInvalidTarget(configPopulateMessage)
	}
	if err := configDecode(map[string]any(c.values()), target); err != nil {
		return newErrDecode(configPopulateMessage, err)
	}
	return nil
}

func (c *configImpl) PopulatePath(
	path string,
	target any,
) error {
	if !configValidTarget(target) {
		return newErrInvalidTarget(configPopulatePathMessage).
			With(ErrorFieldPath, path)
	}
	values := c.values()
	if !values.Has(path) {
		return newErrMissingPath(configPopulatePathMessage, path)
	}
	if err := configDecode(values.Get(path), target); err != nil {
		return newErrDecode(configPopulatePathMessage, err).
			With(ErrorFieldPath, path)
	}
	return nil
}

func (c *configImpl) HasSource(
	id string,
) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	_, ok := c.sources[id]
	return ok
}

func (c *configImpl) AddSource(
	source flam.IConfigSource,
) error {
	if value.IsNil(source) {
		return newErrNilSource(configAddSourceMessage)
	}
	// the source is read before holding the configuration, as its methods
	// may read the configuration
	id, priority := source.Id(), source.Priority()
	values := source.Bag()
	if err := configCheck(values); err != nil {
		return newErrSourceValues(configAddSourceMessage, id, err)
	}

	return c.update(func() error {
		if _, ok := c.sources[id]; ok {
			return newErrDuplicateSource(configAddSourceMessage, id)
		}
		sources := maps.Clone(c.sources)
		sources[id] = &configSource{id: id, source: source, priority: priority, values: values}
		c.remerge(c.defaults, sources)
		return nil
	})
}

func (c *configImpl) RemoveSource(
	id string,
) error {
	return c.update(func() error {
		if _, ok := c.sources[id]; !ok {
			return newErrUnknownSource(configRemoveSourceMessage, id)
		}
		sources := maps.Clone(c.sources)
		delete(sources, id)
		c.remerge(c.defaults, sources)
		return nil
	})
}

func (c *configImpl) HasObserver(
	id string,
	path string,
) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	observer, ok := c.observers[id]
	return ok && observer.path == path
}

func (c *configImpl) AddObserver(
	id string,
	path string,
	callback flam.ConfigObserver,
) error {
	if callback == nil {
		return newErrNilObserver(configAddObserverMessage, id, path)
	}
	if !container.ValidPath(path) {
		return newErrObserverPath(configAddObserverMessage, id, path)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.observers[id]; ok {
		return newErrDuplicateObserver(configAddObserverMessage, id)
	}
	c.observers[id] = &configObserver{path: path, callback: callback}
	return nil
}

func (c *configImpl) RemoveObserver(
	id string,
) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.observers[id]; !ok {
		return newErrUnknownObserver(configRemoveObserverMessage, id)
	}
	delete(c.observers, id)
	return nil
}

func (c *configImpl) AddDefaults(
	defaults flam.IBag,
) error {
	// a deep copy, as the merge copies only the bags, maps of any and
	// slices it knows, so that the caller changing its values afterwards
	// changes no default
	copied := container.Bag{}
	if !value.IsNil(defaults) {
		for key, item := range defaults.All() {
			copied[key] = configCopy(item)
		}
	}
	return c.update(func() error {
		merged := container.Bag{}
		if _, err := merged.Merge("", copied); err != nil {
			return err
		}
		// the defaults already added are merged last, so that they win
		_, _ = merged.Merge("", c.defaults)
		c.remerge(merged, c.sources)
		return nil
	})
}

func (c *configImpl) ReloadSources() error {
	calls, err := c.reload()
	// the observers are called once the reloads are over, so that they may
	// reload the sources themselves
	configNotify(calls)
	return err
}

// messages of the errors returned by the configuration
const (
	configPopulateMessage       = "unable to populate from config"
	configPopulatePathMessage   = "unable to populate from config path"
	configAddSourceMessage      = "unable to add config source"
	configRemoveSourceMessage   = "unable to remove config source"
	configReloadSourcesMessage  = "unable to reload config source"
	configAddObserverMessage    = "unable to add config observer"
	configRemoveObserverMessage = "unable to remove config observer"
)

// reload reloads the reloadable sources, one reload at a time, then
// replaces the values of those that changed, unless removed meanwhile,
// returning the calls of the observers whose value changed, and the errors
// of the reloads, joined. The sources are reloaded without holding the
// configuration, as a reload may take long, such as a request to a remote
// endpoint.
func (c *configImpl) reload() ([]configCall, error) {
	c.reloadMu.Lock()
	defer c.reloadMu.Unlock()

	c.mu.RLock()
	entries := configSorted(c.sources)
	c.mu.RUnlock()

	var errs []error
	// the values reloaded, by the entry of their source
	reloaded := map[string]configReloaded{}
	for _, entry := range entries {
		source, ok := entry.source.(flam.IConfigReloadableSource)
		if !ok {
			continue
		}
		changed, err := source.Reload()
		if err != nil {
			errs = append(errs, newErrReload(configReloadSourcesMessage, entry.id, err))
			continue
		}
		if !changed {
			continue
		}
		values := source.Bag()
		if err := configCheck(values); err != nil {
			errs = append(errs, newErrSourceValues(configReloadSourcesMessage, entry.id, err))
			continue
		}
		reloaded[entry.id] = configReloaded{entry: entry, values: values}
	}
	if len(reloaded) == 0 {
		return nil, errors.Join(errs...)
	}

	calls, _ := c.apply(func() error {
		sources := maps.Clone(c.sources)
		for id, current := range sources {
			// a source removed meanwhile is skipped, but not when added
			// again, as the values reloaded are as recent as those it was
			// added with
			r, ok := reloaded[id]
			if ok && (current == r.entry || configSameSource(current.source, r.entry.source)) {
				sources[id] = &configSource{id: id, source: current.source, priority: current.priority, values: r.values}
			}
		}
		c.remerge(c.defaults, sources)
		return nil
	})
	return calls, errors.Join(errs...)
}

// configScopedKey is the key of the value of the bags returned by scoped.
const configScopedKey = "value"

// values returns the bag of the merged layers, which is never changed, so
// that it can be read once the configuration is released.
func (c *configImpl) values() container.Bag {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.merged
}

// scoped returns a new bag holding, at configScopedKey, a copy of the value
// stored at the path, or nothing when no value is stored there, so that the
// getters of the bag read a copy.
func (c *configImpl) scoped(
	path string,
) container.Bag {
	values := c.values()
	if !values.Has(path) {
		return container.Bag{}
	}
	return container.Bag{configScopedKey: configCopy(values.Get(path))}
}

// update applies the change, as apply does, then, once the configuration is
// released, calls the observers whose value changed, in observer id order.
func (c *configImpl) update(
	change func() error,
) error {
	calls, err := c.apply(change)
	if err != nil {
		return err
	}
	configNotify(calls)
	return nil
}

// apply applies the change while holding the configuration, and returns
// the calls of the observers whose value changed, in observer id order, to
// be made by configNotify once nothing is held. A failed change must leave
// the configuration unchanged.
func (c *configImpl) apply(
	change func() error,
) ([]configCall, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	old := c.merged
	if err := change(); err != nil {
		return nil, err
	}
	return c.calls(old, c.merged), nil
}

// remerge makes the defaults and the sources those of the configuration,
// and merges them with the Set layer. It must be called while holding the
// configuration.
func (c *configImpl) remerge(
	defaults container.Bag,
	sources map[string]*configSource,
) {
	c.defaults, c.sources = defaults, sources
	c.merged = configMerge(defaults, sources, c.set)
}

// calls returns the calls of the observers whose value differs between
// the old and new merged bags, in observer id order. It must be called
// while holding the configuration.
func (c *configImpl) calls(
	old container.Bag,
	newValues container.Bag,
) []configCall {
	var calls []configCall
	for _, id := range slices.Sorted(maps.Keys(c.observers)) {
		observer := c.observers[id]
		before := old.Get(observer.path)
		after := newValues.Get(observer.path)
		if !value.Equal(before, after) {
			calls = append(calls, configCall{callback: observer.callback, old: before, newValue: after})
		}
	}
	return calls
}

// configNotify makes the calls of the observers, with copies of the
// values, so that the callbacks cannot change the configuration.
func configNotify(
	calls []configCall,
) {
	for _, call := range calls {
		call.callback(configCopy(call.old), configCopy(call.newValue))
	}
}

// configSameSource reports whether the sources are the same, those that
// cannot be compared never being the same, so that such a source added
// again while reloaded keeps the values it is added with. The values are
// checked, not their types, as a comparable struct type holding an
// interface field holding a map, for one, panics when compared.
func configSameSource(
	a flam.IConfigSource,
	b flam.IConfigSource,
) bool {
	if !reflect.ValueOf(a).Comparable() || !reflect.ValueOf(b).Comparable() {
		return false
	}
	return a == b
}

// configMerge returns a new bag merging the defaults, the values of the
// sources, by increasing priority and id, and the Set layer. The layers
// are checked when they change, so that their merge cannot fail: merged
// at the root, a layer never conflicts with another.
func configMerge(
	defaults container.Bag,
	sources map[string]*configSource,
	set container.Bag,
) container.Bag {
	merged := container.Bag{}
	layers := []flam.IBag{defaults}
	for _, entry := range configSorted(sources) {
		layers = append(layers, entry.values)
	}
	for _, layer := range append(layers, set) {
		_, _ = merged.Merge("", layer)
	}
	return merged
}

// configSorted returns the sources by increasing priority and, for equal
// priorities, by id.
func configSorted(
	sources map[string]*configSource,
) []*configSource {
	return slices.SortedFunc(maps.Values(sources), func(a, b *configSource) int {
		if c := cmp.Compare(a.priority, b.priority); c != 0 {
			return c
		}
		return strings.Compare(a.id, b.id)
	})
}

// configCheck returns the error of the merge of the values, such as for a
// key that is not a valid path: merged at the root, the values of a layer
// cannot conflict with the other layers.
func configCheck(
	values flam.IBag,
) error {
	_, err := container.Bag{}.Merge("", values)
	return err
}

// configCopy returns a deep copy of the value: the maps and slices, of any
// type, are copied at any depth, through the interfaces they hold, keeping
// their dynamic type and nil-ness, while the other values, pointers and
// structs included, are copied as Go assigns them.
func configCopy(
	value any,
) any {
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Map:
		if v.IsNil() {
			return value
		}
		result := reflect.MakeMapWithSize(v.Type(), v.Len())
		for entry := v.MapRange(); entry.Next(); {
			result.SetMapIndex(entry.Key(), configCopyItem(entry.Value()))
		}
		return result.Interface()
	case reflect.Slice:
		if v.IsNil() {
			return value
		}
		result := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
		for i := range v.Len() {
			result.Index(i).Set(configCopyItem(v.Index(i)))
		}
		return result.Interface()
	}
	return value
}

// configCopyItem returns a deep copy of an item of a map or slice, of the
// item type, as configCopy copies a value.
func configCopyItem(
	item reflect.Value,
) reflect.Value {
	if item.Kind() == reflect.Interface && item.IsNil() {
		return item
	}
	result := reflect.New(item.Type()).Elem()
	result.Set(reflect.ValueOf(configCopy(item.Interface())))
	return result
}

// configFirst returns the first default, or the zero value when none is
// given.
func configFirst[T any](
	def []T,
) T {
	if len(def) > 0 {
		return def[0]
	}
	var zero T
	return zero
}

// configStrings returns the bag as a new map[string]string, or false when
// one of its values is not a string.
func configStrings(
	bag map[string]any,
) (map[string]string, bool) {
	result := make(map[string]string, len(bag))
	for key, item := range bag {
		str, ok := item.(string)
		if !ok {
			return nil, false
		}
		result[key] = str
	}
	return result, true
}

// configValidTarget reports whether the target is a non-nil pointer.
func configValidTarget(
	target any,
) bool {
	v := reflect.ValueOf(target)
	return v.Kind() == reflect.Pointer && !v.IsNil()
}

// configDecode decodes the value into the target through encoding/json, or
// returns the json error. The infinite and NaN floats, which JSON cannot
// hold, are encoded as null, which leaves their target unchanged, so that
// such a value fails no decoding, even of a target not reading it.
func configDecode(
	value any,
	target any,
) error {
	data, err := json.Marshal(configFinite(reflect.ValueOf(value)))
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

// configFinite returns the value with its infinite and NaN floats, through
// its maps and slices, replaced by nil, sharing what holds none.
func configFinite(
	v reflect.Value,
) any {
	switch v.Kind() {
	case reflect.Invalid:
		return nil
	case reflect.Interface, reflect.Pointer:
		if v.IsNil() {
			return v.Interface()
		}
		if v.Kind() == reflect.Interface {
			return configFinite(v.Elem())
		}
	case reflect.Float32, reflect.Float64:
		if f := v.Float(); math.IsInf(f, 0) || math.IsNaN(f) {
			return nil
		}
	case reflect.Map:
		if v.Type().Key().Kind() == reflect.String && !v.IsNil() {
			result := make(map[string]any, v.Len())
			for entry := v.MapRange(); entry.Next(); {
				result[entry.Key().String()] = configFinite(entry.Value())
			}
			return result
		}
	case reflect.Slice:
		if v.Type().Elem().Kind() != reflect.Uint8 && !v.IsNil() { // []byte encodes as base64
			result := make([]any, v.Len())
			for i := range v.Len() {
				result[i] = configFinite(v.Index(i))
			}
			return result
		}
	}
	return v.Interface()
}
