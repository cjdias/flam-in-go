package tests

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cjdias/flam-in-go"
)

func Test_Error_NewError(t *testing.T) {
	t.Run("should create error with a simple message", func(t *testing.T) {
		msg := "test error"
		e := flam.NewError(msg)
		assert.Equal(t, msg, e.Error())
		assert.NotNil(t, e.Context())
	})

	t.Run("should create error with a message and context", func(t *testing.T) {
		msg := "test error"
		ctx := flam.Bag{"key": "value"}
		e := flam.NewError(msg, ctx)
		assert.Equal(t, msg, e.Error())
		assert.Equal(t, ctx, *e.Context())
	})
}

func Test_Error_NewErrorFrom(t *testing.T) {
	baseErr := errors.New("base error")

	t.Run("should wrapping an error with a message", func(t *testing.T) {
		msg := "additional context"
		e := flam.NewErrorFrom(baseErr, msg)
		assert.ErrorIs(t, e, baseErr)
		assert.Equal(t, "base error: additional context", e.Error())
	})

	t.Run("should wrapping an error with a message and context", func(t *testing.T) {
		msg := "context"
		ctx := flam.Bag{"key": "value"}
		e := flam.NewErrorFrom(baseErr, msg, ctx)
		assert.ErrorIs(t, e, baseErr)
		assert.Equal(t, "base error: context", e.Error())
		assert.Equal(t, ctx, *e.Context())
	})
}

func Test_Error_GetCode(t *testing.T) {
	assert.Equal(t, 404, flam.NewError("test").SetCode(404).GetCode())
}

func Test_Error_SetCode(t *testing.T) {
	e := flam.NewError("test")
	assert.Same(t, e, e.SetCode(500))
	assert.Equal(t, 500, e.GetCode())
}

func Test_Error_Context(t *testing.T) {
	ctx := flam.NewError("test").Set("detail", "not found").Context()
	assert.Equal(t, &flam.Bag{"detail": "not found"}, ctx)
}

func Test_Error_Set(t *testing.T) {
	e := flam.NewError("test")
	assert.Same(t, e, e.Set("user.id", 123))
	assert.Equal(t, 123, e.Get("user.id"))
}

func Test_Error_Get(t *testing.T) {
	e := flam.NewError("test").Set("detail", "found")

	t.Run("should retrieve stored valid", func(t *testing.T) {
		assert.Equal(t, "found", e.Get("detail"))
	})

	t.Run("should return nil if the value is not present", func(t *testing.T) {
		assert.Nil(t, e.Get("non.existent"))
	})

	t.Run("should return given default if the value is not present", func(t *testing.T) {
		assert.Equal(t, "default", e.Get("non.existent", "default"))
	})
}

func Test_Error_Unwrap(t *testing.T) {
	base := errors.New("base")

	assert.ErrorIs(t, flam.NewErrorFrom(base, "wrapped").Unwrap(), base)
}
