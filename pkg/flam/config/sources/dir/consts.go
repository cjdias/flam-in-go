package dir

import (
	"github.com/cjdias/flam-in-go/pkg/flam/disk/disks/local"
)

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.config.sources.dir"

// Driver is the driver name of the dir sources, held by the
// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver] field
// of their configuration.
const Driver = "flam.config.sources.driver.dir"

// Fields of a dir source configuration.
const (
	// ConfigFieldPriority is the field holding the int priority of the
	// source.
	ConfigFieldPriority = "priority"

	// ConfigFieldReload is the field holding the bool telling whether the
	// source is reloaded: when false, the source does not implement
	// [github.com/cjdias/flam-in-go/pkg/flam.IConfigReloadableSource], so
	// its values never change.
	ConfigFieldReload = "reload"

	// ConfigFieldDisk is the field holding the id of the disk of the
	// directory.
	ConfigFieldDisk = "disk"

	// ConfigFieldPath is the required field holding the path of the
	// directory on its disk.
	ConfigFieldPath = "path"

	// ConfigFieldRecursive is the field holding the bool telling whether
	// the files of the sub directories are read too.
	ConfigFieldRecursive = "recursive"

	// ConfigFieldParser is the field holding the id of the parser of every
	// file. When empty, each file is parsed with the parser named after its
	// extension.
	ConfigFieldParser = "parser"
)

// Configuration paths of the values taken by the fields missing from a
// source configuration.
const (
	// DefaultPriorityConfigPath is the path of the default of
	// [ConfigFieldPriority], 0 unless changed.
	DefaultPriorityConfigPath = "flam.config.defaults.sources.dir.priority"

	// DefaultReloadConfigPath is the path of the default of
	// [ConfigFieldReload], true unless changed.
	DefaultReloadConfigPath = "flam.config.defaults.sources.dir.reload"

	// DefaultDiskConfigPath is the path of the default of
	// [ConfigFieldDisk], the disk of
	// [github.com/cjdias/flam-in-go/pkg/flam/disk/disks/local.Id] unless
	// changed.
	DefaultDiskConfigPath = "flam.config.defaults.sources.dir.disk"

	// DefaultRecursiveConfigPath is the path of the default of
	// [ConfigFieldRecursive], false unless changed.
	DefaultRecursiveConfigPath = "flam.config.defaults.sources.dir.recursive"

	// DefaultParserConfigPath is the path of the default of
	// [ConfigFieldParser], empty, so named after the file extensions, unless
	// changed.
	DefaultParserConfigPath = "flam.config.defaults.sources.dir.parser"
)

// Context fields of the [*flam.Error] values returned by the creator and
// the sources.
const (
	// ErrorFieldID holds the id of the source.
	ErrorFieldID = "id"

	// ErrorFieldField holds the name of an invalid configuration field.
	ErrorFieldField = "field"

	// ErrorFieldDisk holds the id of the disk of the directory.
	ErrorFieldDisk = "disk"

	// ErrorFieldPath holds the path of the directory, or of the file, in
	// error.
	ErrorFieldPath = "path"

	// ErrorFieldParser holds the id of a parser.
	ErrorFieldParser = "parser"
)

// Defaults of the fields of a source configuration.
const (
	defaultPriority  = 0
	defaultReload    = true
	defaultDisk      = local.Id
	defaultRecursive = false
	defaultParser    = ""
)
