// Package static hides the reloading of the configuration sources whose
// reload is deactivated by their configuration.
package static

import (
	"github.com/cjdias/flam-in-go/pkg/flam"
)

// New returns the source as a [flam.IConfigSource] only: the result does
// not implement [flam.IConfigReloadableSource], even when the source does,
// so that it is never reloaded.
func New(
	source flam.IConfigSource,
) flam.IConfigSource {
	return &staticSource{IConfigSource: source}
}

// staticSource exposes the methods of [flam.IConfigSource] of the source it
// embeds, and those only.
type staticSource struct {
	flam.IConfigSource
}
