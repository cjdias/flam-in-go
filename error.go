package flam

import (
	"errors"
	"fmt"
)

type Error interface {
	error

	Unwrap() error
	GetCode() int
	SetCode(code int) Error
	Context() *Bag
	Set(path string, value any) Error
	Get(path string, def ...any) any
}

type err struct {
	base    error
	code    int
	context Bag
}

var _ error = &err{}
var _ Error = &err{}

func NewError(
	msg string,
	ctx ...Bag,
) Error {
	return &err{
		base:    errors.New(msg),
		context: mergeContext(ctx...)}
}

func NewErrorFrom(
	e error,
	msg string,
	ctx ...Bag,
) Error {
	return &err{
		base:    fmt.Errorf("%w: %s", e, msg),
		context: mergeContext(ctx...)}
}

func (e *err) Error() string {
	return e.base.Error()
}

func (e *err) Unwrap() error {
	return errors.Unwrap(e.base)
}

func (e *err) GetCode() int {
	return e.code
}

func (e *err) SetCode(
	code int,
) Error {
	e.code = code

	return e
}

func (e *err) Context() *Bag {
	clone := e.context.Clone()
	return &clone
}

func (e *err) Set(
	path string,
	value any,
) Error {
	// Error ignored - setting context values, shouldn't block error handling
	_ = e.context.Set(path, value)

	return e
}

func (e *err) Get(
	path string,
	def ...any,
) any {
	return e.context.Get(path, def...)
}
