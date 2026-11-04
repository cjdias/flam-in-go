package openapi

import (
	"errors"
)

var (
	// ErrRouter is the error wrapped by the error returned from the
	// Register method of the register when the router it is given does
	// not list its endpoints, as a
	// [github.com/cjdias/flam-in-go/pkg/flam.IRestEndpointLister].
	ErrRouter = errors.New("rest router does not list its endpoints")
)
