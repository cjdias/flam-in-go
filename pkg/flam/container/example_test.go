package container_test

import (
	"fmt"

	"github.com/cjdias/flam-in-go/pkg/flam/container"
)

func ExampleBag() {
	bag := container.Bag{}
	_ = bag.Set("server.host", "localhost")
	_ = bag.Set("server.port", 8080)

	fmt.Println(bag.String("server.host"))
	fmt.Println(bag.Int("server.port"))
	fmt.Println(bag.Int("server.timeout", 30))
	// Output:
	// localhost
	// 8080
	// 30
}
