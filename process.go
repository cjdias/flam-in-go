package flam

import "io"

type Process interface {
	io.Closer

	Id() string
	IsRunning() bool
	Run() error
	Terminate()
}
