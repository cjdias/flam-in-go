package full

import (
	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/app"
)

// New returns the [flam.IApp] of
// [github.com/cjdias/flam-in-go/pkg/flam/app.New], with the values, and
// the bundle of [NewBundle] registered after its core bundles. A failure
// to register it is kept by the application, failing its boot, as for
// the core bundles.
func New(
	values flam.IBag,
) flam.IApp {
	a := app.New(values)
	_ = a.RegisterBundle(NewBundle()) // a failure is kept, failing the boot
	return a
}
