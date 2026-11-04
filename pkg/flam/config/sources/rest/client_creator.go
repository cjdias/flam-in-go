package rest

import (
	"net/http"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// newClientCreator returns the default [flam.IConfigRestClientCreator]
// provided by the bundle returned from [NewBundle].
func newClientCreator() flam.IConfigRestClientCreator {
	return &clientCreator{}
}

// clientCreator is the default [flam.IConfigRestClientCreator], creating a
// new [http.Client] with a timeout of 30 seconds on each call.
type clientCreator struct{}

func (c *clientCreator) Create() (flam.IConfigRestClient, error) {
	return &http.Client{Timeout: defaultClientTimeout}, nil
}
