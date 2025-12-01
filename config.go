package flam

import (
	"reflect"
	"sync"
	"time"
)

type ConfigObserver func(old, new any)

type Config interface {
	Entries() []string
	Has(path string) bool
	Get(path string, def ...any) any
	Bool(path string, def ...bool) bool
	Int(path string, def ...int) int
	Int8(path string, def ...int8) int8
	Int16(path string, def ...int16) int16
	Int32(path string, def ...int32) int32
	Int64(path string, def ...int64) int64
	Uint(path string, def ...uint) uint
	Uint8(path string, def ...uint8) uint8
	Uint16(path string, def ...uint16) uint16
	Uint32(path string, def ...uint32) uint32
	Uint64(path string, def ...uint64) uint64
	Float32(path string, def ...float32) float32
	Float64(path string, def ...float64) float64
	String(path string, def ...string) string
	StringMap(path string, def ...map[string]any) map[string]any
	StringMapString(path string, def ...map[string]string) map[string]string
	Slice(path string, def ...[]any) []any
	StringSlice(path string, def ...[]string) []string
	Duration(path string, def ...time.Duration) time.Duration
	Bag(path string, def ...Bag) Bag
	Set(path string, value any) error
	Populate(target any, path ...string) error

	HasObserver(id, path string) bool
	AddObserver(id, path string, callback ConfigObserver) error
	RemoveObserver(id string) error
}

type configObserverReg struct {
	current   any
	callbacks map[string]ConfigObserver
}

type config struct {
	locker       sync.Locker
	sourcesBag   Bag
	managerBag   Bag
	aggregateBag Bag
	observerRegs map[string]configObserverReg
}

var _ Config = (*config)(nil)

func newConfig() *config {
	return &config{
		locker:       &sync.Mutex{},
		sourcesBag:   Bag{},
		managerBag:   Bag{},
		aggregateBag: Bag{},
		observerRegs: map[string]configObserverReg{}}
}

func (config *config) Entries() []string {
	return config.aggregateBag.Entries()
}

func (config *config) Has(
	path string,
) bool {
	config.locker.Lock()
	defer config.locker.Unlock()

	return config.aggregateBag.Has(path)
}

func (config *config) Get(
	path string,
	def ...any,
) any {
	config.locker.Lock()
	defer config.locker.Unlock()

	return config.aggregateBag.Get(path, def...)
}

func (config *config) Bool(
	path string,
	def ...bool,
) bool {
	config.locker.Lock()
	defer config.locker.Unlock()

	return config.aggregateBag.Bool(path, def...)
}

func (config *config) Int(
	path string,
	def ...int,
) int {
	config.locker.Lock()
	defer config.locker.Unlock()

	return config.aggregateBag.Int(path, def...)
}

func (config *config) Int8(
	path string,
	def ...int8,
) int8 {
	config.locker.Lock()
	defer config.locker.Unlock()

	return config.aggregateBag.Int8(path, def...)
}

func (config *config) Int16(
	path string,
	def ...int16,
) int16 {
	config.locker.Lock()
	defer config.locker.Unlock()

	return config.aggregateBag.Int16(path, def...)
}

func (config *config) Int32(
	path string,
	def ...int32,
) int32 {
	config.locker.Lock()
	defer config.locker.Unlock()

	return config.aggregateBag.Int32(path, def...)
}

func (config *config) Int64(
	path string,
	def ...int64,
) int64 {
	config.locker.Lock()
	defer config.locker.Unlock()

	return config.aggregateBag.Int64(path, def...)
}

func (config *config) Uint(
	path string,
	def ...uint,
) uint {
	config.locker.Lock()
	defer config.locker.Unlock()

	return config.aggregateBag.Uint(path, def...)
}

func (config *config) Uint8(
	path string,
	def ...uint8,
) uint8 {
	config.locker.Lock()
	defer config.locker.Unlock()

	return config.aggregateBag.Uint8(path, def...)
}

func (config *config) Uint16(
	path string,
	def ...uint16,
) uint16 {
	config.locker.Lock()
	defer config.locker.Unlock()

	return config.aggregateBag.Uint16(path, def...)
}

func (config *config) Uint32(
	path string,
	def ...uint32,
) uint32 {
	config.locker.Lock()
	defer config.locker.Unlock()

	return config.aggregateBag.Uint32(path, def...)
}

func (config *config) Uint64(
	path string,
	def ...uint64,
) uint64 {
	config.locker.Lock()
	defer config.locker.Unlock()

	return config.aggregateBag.Uint64(path, def...)
}

func (config *config) Float32(
	path string,
	def ...float32,
) float32 {
	config.locker.Lock()
	defer config.locker.Unlock()

	return config.aggregateBag.Float32(path, def...)
}

func (config *config) Float64(
	path string,
	def ...float64,
) float64 {
	config.locker.Lock()
	defer config.locker.Unlock()

	return config.aggregateBag.Float64(path, def...)
}

func (config *config) String(
	path string,
	def ...string,
) string {
	config.locker.Lock()
	defer config.locker.Unlock()

	return config.aggregateBag.String(path, def...)
}

func (config *config) StringMap(
	path string,
	def ...map[string]any,
) map[string]any {
	config.locker.Lock()
	defer config.locker.Unlock()

	return config.aggregateBag.StringMap(path, def...)
}

func (config *config) StringMapString(
	path string,
	def ...map[string]string,
) map[string]string {
	config.locker.Lock()
	defer config.locker.Unlock()

	return config.aggregateBag.StringMapString(path, def...)
}

func (config *config) Slice(
	path string,
	def ...[]any,
) []any {
	config.locker.Lock()
	defer config.locker.Unlock()

	return config.aggregateBag.Slice(path, def...)
}

func (config *config) StringSlice(
	path string,
	def ...[]string,
) []string {
	config.locker.Lock()
	defer config.locker.Unlock()

	return config.aggregateBag.StringSlice(path, def...)
}

func (config *config) Duration(
	path string,
	def ...time.Duration,
) time.Duration {
	config.locker.Lock()
	defer config.locker.Unlock()

	return config.aggregateBag.Duration(path, def...)
}

func (config *config) Bag(
	path string,
	def ...Bag,
) Bag {
	config.locker.Lock()
	defer config.locker.Unlock()

	return config.aggregateBag.Bag(path, def...)
}

func (config *config) Set(
	path string,
	value any,
) error {
	if e := config.managerBag.Set(path, value); e != nil {
		return e
	}

	config.locker.Lock()
	defer config.locker.Unlock()

	config.rebuild()

	return nil
}

func (config *config) Populate(target any, path ...string) error {
	config.locker.Lock()
	defer config.locker.Unlock()

	return config.aggregateBag.Populate(target, path...)
}

func (config *config) HasObserver(
	id,
	path string,
) bool {
	if reg, ok := config.observerRegs[path]; ok {
		if _, ok := reg.callbacks[id]; ok {
			return true
		}
	}

	return false
}

func (config *config) AddObserver(
	id,
	path string,
	observer ConfigObserver,
) error {
	config.locker.Lock()
	defer config.locker.Unlock()

	if observer == nil {
		return newErrNilReference("callback")
	}

	if _, ok := config.observerRegs[path]; !ok {
		config.observerRegs[path] = configObserverReg{
			current:   config.aggregateBag.Get(path),
			callbacks: map[string]ConfigObserver{}}
	} else if _, ok := config.observerRegs[path].callbacks[id]; ok {
		return newErrDuplicateConfigObserver(path, id)
	}

	config.observerRegs[path].callbacks[id] = observer

	return nil
}

func (config *config) RemoveObserver(
	id string,
) error {
	config.locker.Lock()
	defer config.locker.Unlock()

	for _, observer := range config.observerRegs {
		delete(observer.callbacks, id)
	}

	return nil
}

func (config *config) rebuild() {
	config.aggregateBag = config.sourcesBag.Clone()
	config.aggregateBag.Merge(config.managerBag)

	for path, reg := range config.observerRegs {
		val := config.aggregateBag.Get(path, nil)
		if val != nil && !reflect.DeepEqual(reg.current, val) {
			old := reg.current
			config.observerRegs[path] = configObserverReg{
				current:   val,
				callbacks: reg.callbacks}

			for _, callback := range config.observerRegs[path].callbacks {
				callback(old, val)
			}
		}
	}
}
