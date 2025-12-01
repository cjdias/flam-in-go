package flam

import (
	"io"
)

type ConfigParser interface {
	io.Closer

	Parse(reader io.Reader) (Bag, error)
}
