package log

import (
	"github.com/cjdias/flam-in-go/pkg/flam"
)

// newBooter returns the booter creating the streams configured for the
// stream factory. It is a provider registered by the bundle returned from
// [NewBundle].
func newBooter(
	config flam.IConfig,
	streams flam.ILogStreamFactory,
) *booter {
	return &booter{
		config:  config,
		streams: streams,
	}
}

// booter creates, when the application boots, the streams configured at
// [StreamConfigPath], storing them in the stream factory, where the logger
// finds them.
type booter struct {
	config  flam.IConfig
	streams flam.ILogStreamFactory
}

// Boot gets from the stream factory, in id order, each stream configured
// at [StreamConfigPath], unless [BootConfigPath] holds false. It stops at
// the first stream failing to be created, leaving the streams created
// before it stored.
func (b *booter) Boot() error {
	if !b.config.Bool(BootConfigPath, defaultBoot) {
		return nil
	}

	for _, id := range b.streams.Available() {
		if _, err := b.streams.Get(id); err != nil {
			return newErrBoot(booterBootMessage, id, err)
		}
	}
	return nil
}

// booterBootMessage is the message of the errors returned by Boot.
const booterBootMessage = "unable to boot log stream"
