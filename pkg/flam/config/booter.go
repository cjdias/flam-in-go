package config

import (
	"github.com/cjdias/flam-in-go/pkg/flam"
)

// newBooter returns the booter adding the sources created by the source
// factory to the configuration. It is a provider registered by the bundle
// returned from [NewBundle].
func newBooter(
	config flam.IConfig,
	sources flam.IConfigSourceFactory,
) *booter {
	return &booter{
		config:  config,
		sources: sources,
	}
}

// booter adds, when the application boots, the sources configured at
// [SourceConfigPath] to the configuration.
type booter struct {
	config  flam.IConfig
	sources flam.IConfigSourceFactory
}

// Boot gets from the source factory, in id order, each source configured
// at [SourceConfigPath], and adds it to the configuration, unless
// [BootConfigPath] holds false. A source whose id was already added to the
// configuration is skipped. It stops at the first source failing to be
// created or added, leaving the sources added before it.
func (b *booter) Boot() error {
	if !b.config.Bool(BootConfigPath, defaultBoot) {
		return nil
	}

	for _, id := range b.sources.Available() {
		if b.config.HasSource(id) {
			continue
		}
		source, err := b.sources.Get(id)
		if err != nil {
			return newErrBoot(booterBootMessage, id, err)
		}
		if err := b.config.AddSource(source); err != nil {
			return newErrBoot(booterBootMessage, id, err)
		}
	}
	return nil
}

// booterBootMessage is the message of the errors returned by Boot.
const booterBootMessage = "unable to boot config source"
