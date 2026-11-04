package config

import (
	"time"
)

// bundleId is the id of the configuration bundle.
const bundleId = "flam.config"

// Configuration paths and container groups of the parsers and sources.
const (
	// ParserConfigPath is the configuration path holding one parser
	// configuration per parser id, as read by the parser factory.
	ParserConfigPath = "flam.config.parsers"

	// ParserCreatorGroup is the container group the parser bundles provide
	// their [flam.IConfigParserCreator] in, collected by the parser
	// factory.
	ParserCreatorGroup = "flam.config.parsers.creator"

	// SourceConfigPath is the configuration path holding one source
	// configuration per source id, as read by the source factory.
	SourceConfigPath = "flam.config.sources"

	// SourceCreatorGroup is the container group the source bundles provide
	// their [flam.IConfigSourceCreator] in, collected by the source
	// factory.
	SourceCreatorGroup = "flam.config.sources.creator"
)

// Configuration paths of the configuration bundle behaviour.
const (
	// BootConfigPath is the configuration path of a bool telling whether
	// the configuration bundle, when the application boots, creates every
	// source configured at [SourceConfigPath] and adds it to the
	// [flam.IConfig]. It defaults to true.
	BootConfigPath = "flam.config.boot"

	// ReloadPeriodConfigPath is the configuration path of the period, a
	// time.Duration or a string such as "30s", at which the configuration
	// bundle reloads the [flam.IConfigReloadableSource] sources added to the
	// [flam.IConfig], with a recurring [flam.ITrigger] of id
	// [ReloaderTriggerID]. A period of zero or less, the default, or a
	// value that is not a period, disables reloading. The bundle observes
	// the path, with an observer of id [ReloaderTriggerID], and restarts
	// the trigger when the period changes.
	ReloadPeriodConfigPath = "flam.config.reloader.period"

	// ReloaderTriggerID is the id of the trigger reloading the sources,
	// and of the configuration observer of [ReloadPeriodConfigPath].
	ReloaderTriggerID = "flam.config.reloader"
)

// Context fields of the [*flam.Error] values returned by the
// configuration.
const (
	// ErrorFieldID holds the id of a source or of an observer.
	ErrorFieldID = "id"

	// ErrorFieldPath holds a configuration path.
	ErrorFieldPath = "path"

	// ErrorFieldPeriod holds the reload period, a time.Duration.
	ErrorFieldPeriod = "period"
)

// Defaults of the configuration bundle behaviour.
const (
	defaultBoot         = true
	defaultReloadPeriod = time.Duration(0)
)
