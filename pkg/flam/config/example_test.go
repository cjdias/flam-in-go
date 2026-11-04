package config_test

import (
	"fmt"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/app"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
)

func Example() {
	// the configuration bundle is a core bundle, registered by app.New
	a := app.New(container.Bag{"server": container.Bag{"port": 8080}})
	defer func() { _ = a.Close() }()

	err := a.Invoke(func(c flam.IConfig) error {
		if err := c.AddObserver("port", "server.port", func(old, newValue any) {
			fmt.Println("port changed from", old, "to", newValue)
		}); err != nil {
			return err
		}

		fmt.Println(c.Int("server.port"), c.String("server.host", "localhost"))
		if err := c.Set("server.port", 9090); err != nil {
			return err
		}

		var server struct {
			Host string
			Port int
		}
		if err := c.PopulatePath("server", &server); err != nil {
			return err
		}
		fmt.Printf("%+v\n", server)
		return nil
	})
	if err != nil {
		fmt.Println(err)
	}
	// Output:
	// 8080 localhost
	// port changed from 8080 to 9090
	// {Host: Port:9090}
}
