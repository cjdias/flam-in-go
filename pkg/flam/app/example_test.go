package app_test

import (
	"fmt"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/app"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
)

// greeter is a bundle printing a greeting read from the configuration
// when the application runs.
type greeter struct{}

func (greeter) Id() string { return "example.greeter" }

func (greeter) Register(flam.IApp) error { return nil }

func (greeter) Run(a flam.IApp) error {
	return a.Invoke(func(c flam.IConfig) {
		fmt.Println("hello,", c.String("example.name"))
	})
}

func ExampleNew() {
	a := app.New(container.Bag{"example": container.Bag{"name": "flam"}})
	defer func() { _ = a.Close() }()

	if err := a.RegisterBundle(greeter{}); err != nil {
		fmt.Println(err)
		return
	}
	if err := a.Run(); err != nil {
		fmt.Println(err)
	}
	// Output:
	// hello, flam
}
