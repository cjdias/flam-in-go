//go:build !windows && !plan9

package streams

import (
	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/log/streams/syslog"
)

// platformBundles returns the bundles of the drivers of the platform, the
// syslog one on the platforms providing log/syslog.
func platformBundles() []flam.IBundle {
	return []flam.IBundle{syslog.NewBundle()}
}
