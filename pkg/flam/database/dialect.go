package database

import (
	"gorm.io/gorm"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/value"
)

// NewDialect returns the [flam.IDatabaseDialect] of the id over the gorm
// dialectors returned by open, for the dialect creators, such as:
//
//	database.NewDialect(id, func() gorm.Dialector { return sqlite.Open(dsn) })
//
// open is called once here, and once per Gorm call, so that each
// connection is opened with a dialector of its own: the drivers change
// the dialector they open a connection with, such as the server version
// the mysql one detects, which connections sharing one would race on. It
// must return a new, non-nil, dialector on each call. NewDialect fails
// with ErrNilDialector when open is nil or returns nil.
func NewDialect(
	id string,
	open func() gorm.Dialector,
) (flam.IDatabaseDialect, error) {
	if open == nil {
		return nil, newErrNil(ErrNilDialector, dialectNewMessage, id)
	}
	dialector := open()
	if value.IsNil(dialector) {
		return nil, newErrNil(ErrNilDialector, dialectNewMessage, id)
	}
	return &dialect{Dialector: dialector, id: id, open: open}, nil
}

// dialect is the [flam.IDatabaseDialect] returned by [NewDialect], its
// embedded dialector, never opened, answering the dialector methods.
type dialect struct {
	gorm.Dialector

	id   string
	open func() gorm.Dialector
}

func (d *dialect) Id() string {
	return d.id
}

func (d *dialect) Gorm() gorm.Dialector {
	return d.open()
}

// dialectNewMessage is the message of the errors returned by NewDialect.
const dialectNewMessage = "unable to create database dialect"
