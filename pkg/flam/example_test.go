package flam_test

import (
	"errors"
	"fmt"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

func ExampleNewWrappedError() {
	errNotFound := errors.New("not found")

	err := flam.NewWrappedError(errNotFound, "unable to load user").
		With("id", 42)

	fmt.Println(err)
	fmt.Println(errors.Is(err, errNotFound))
	fmt.Println(err.Context()["id"])
	// Output:
	// unable to load user: not found
	// true
	// 42
}
