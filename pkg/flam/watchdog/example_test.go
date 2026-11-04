package watchdog_test

import (
	"context"
	"fmt"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/app"
	"github.com/cjdias/flam-in-go/pkg/flam/watchdog"
)

// job is a process doing its work once, then ending.
type job struct{}

func (job) Id() string      { return "job" }
func (job) IsRunning() bool { return false }
func (job) Close() error    { return nil }

func (job) Run(context.Context) error {
	fmt.Println("job done")
	return nil
}

func ExampleNewBundle() {
	a := app.New(nil)
	defer func() { _ = a.Close() }()

	if err := a.RegisterBundle(watchdog.NewBundle()); err != nil {
		fmt.Println(err)
		return
	}
	if err := a.Provide(func() flam.IWatchdogProcess { return job{} }, dig.Group(watchdog.ProcessGroup)); err != nil {
		fmt.Println(err)
		return
	}
	// runs the active processes until they end, or until an interrupt
	if err := a.Run(); err != nil {
		fmt.Println(err)
	}
	// Output:
	// job done
}
