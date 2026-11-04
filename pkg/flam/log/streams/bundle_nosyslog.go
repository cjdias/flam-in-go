//go:build windows || plan9

package streams

import (
	"github.com/cjdias/flam-in-go/pkg/flam"
)

// platformBundles returns the bundles of the drivers of the platform, none
// on the platforms without log/syslog.
func platformBundles() []flam.IBundle {
	return nil
}
