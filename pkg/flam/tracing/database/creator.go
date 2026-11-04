package database

import (
	"errors"
	"io"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/dig"
	"gorm.io/gorm"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/value"
)

// creatorsIn are the connection creators the decorator decorates, of the
// ConnectionCreatorGroup group of the database feature, and the tracer
// provider of their plugin, taken from the container. The group tag
// repeats the constant, as a struct tag cannot use one.
type creatorsIn struct {
	dig.In

	Provider trace.TracerProvider
	Creators []flam.IDatabaseConnectionCreator `group:"flam.database.connections.creator"`
}

// creatorsOut are the decorated connection creators, replacing the ones of
// the group.
type creatorsOut struct {
	dig.Out

	Creators []flam.IDatabaseConnectionCreator `group:"flam.database.connections.creator"`
}

// decorateCreators returns the connection creators of the group, each
// adding the plugin of the tracer provider to the connections it creates.
// It is the decorator registered by the bundle returned from [NewBundle].
func decorateCreators(
	in creatorsIn,
) creatorsOut {
	plugin := NewPlugin(in.Provider)
	creators := make([]flam.IDatabaseConnectionCreator, len(in.Creators))
	for i, c := range in.Creators {
		creators[i] = &creator{IDatabaseConnectionCreator: c, plugin: plugin}
	}
	return creatorsOut{Creators: creators}
}

// creator is the [flam.IDatabaseConnectionCreator] decorating the one it
// embeds, adding the plugin to the connections it creates.
type creator struct {
	flam.IDatabaseConnectionCreator

	plugin gorm.Plugin
}

// Create returns the connection created by the decorated creator, the
// plugin added, a connection having it already being returned as it is.
// It closes the connection, when it implements io.Closer, and fails, when
// the plugin cannot be added.
func (c *creator) Create(
	bag flam.IBag,
) (flam.IDatabaseConnection, error) {
	conn, err := c.IDatabaseConnectionCreator.Create(bag)
	if err != nil || value.IsNil(conn) {
		return conn, err
	}
	if err := conn.Use(c.plugin); err != nil && !errors.Is(err, gorm.ErrRegistered) {
		if closer, ok := conn.(io.Closer); ok {
			_ = closer.Close()
		}
		return nil, newErrPlugin(creatorCreateMessage, bag.String(factory.ConfigFieldId), err)
	}
	return conn, nil
}

// creatorCreateMessage is the message of the errors returned by Create.
const creatorCreateMessage = "unable to create traced database connection"
