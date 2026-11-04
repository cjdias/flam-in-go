package redis

import (
	"errors"
	"fmt"
	"os"
	"sync/atomic"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/broker/brokers/internal/base"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
)

// creatorArgs are the dependencies of the creator, taken from the
// container, the connection factory being optional so that the other
// drivers work without the redis bundle.
type creatorArgs struct {
	dig.In

	Config      flam.IConfig
	Connections flam.IRedisConnectionFactory `optional:"true"`
}

// newCreator returns the [flam.IBrokerCreator] of the redis brokers,
// reading the defaults of the missing fields from the configuration. It is
// the provider registered by the bundle returned from [NewBundle].
func newCreator(
	args creatorArgs,
) flam.IBrokerCreator {
	return &creator{config: args.Config, connections: args.Connections}
}

// creator is the [flam.IBrokerCreator] of the configurations with the
// [Driver] driver.
type creator struct {
	config      flam.IConfig
	connections flam.IRedisConnectionFactory
}

func (c *creator) Accept(
	bag flam.IBag,
) bool {
	return bag.String(factory.ConfigFieldDriver) == Driver
}

// Create returns the broker of the fields, failing at the first invalid
// one, over the redis connection of its connection field, taken from the
// connection factory, which keeps owning it.
func (c *creator) Create(
	bag flam.IBag,
) (flam.IBroker, error) {
	id := bag.String(factory.ConfigFieldId)
	r := base.NewReader(c.config, bag)

	connection := r.String(ConfigFieldConnection, DefaultConnectionConfigPath, defaultConnection, true)
	b := &redisBroker{
		id:        id,
		mode:      r.OneOf(ConfigFieldMode, DefaultModeConfigPath, defaultMode, ModePubSub, ModeStreams),
		group:     r.String(ConfigFieldGroup, "", "", false),
		consumer:  r.String(ConfigFieldConsumer, "", "", false),
		start:     r.OneOf(ConfigFieldStart, DefaultStartConfigPath, defaultStart, StartNew, StartAll),
		batch:     int64(r.Int(ConfigFieldBatch, DefaultBatchConfigPath, defaultBatch, 1)),
		block:     r.Duration(ConfigFieldBlock, DefaultBlockConfigPath, defaultBlock),
		claimIdle: r.Duration(ConfigFieldClaimIdle, DefaultClaimIdleConfigPath, defaultClaimIdle),
		maxLen:    int64(r.Int(ConfigFieldMaxLen, DefaultMaxLenConfigPath, defaultMaxLen, 0)),
		registry:  base.NewRegistry(id),
	}
	r.Check(ConfigFieldBlock, b.block > 0) // a zero block waits forever
	if field := r.Invalid(); field != "" {
		return nil, base.NewErrInvalidField(ErrInvalidField, creatorCreateMessage, id, field)
	}
	if b.group == "" {
		b.group = id
	}

	var err error
	if c.connections == nil {
		err = errNoFactory
	} else {
		b.conn, err = c.connections.Get(connection)
	}
	if err != nil {
		return nil, base.NewErr(ErrConnection, creatorCreateMessage, id, err).
			With(ErrorFieldConnection, connection)
	}
	return b, nil
}

// creatorCreateMessage is the message of the errors returned by Create.
const creatorCreateMessage = "unable to create redis broker"

// errNoFactory is the cause of the ErrConnection errors when the container
// provides no connection factory.
var errNoFactory = errors.New("no redis connection factory, register the redis bundle")

// consumers numbers the consumers named by the brokers, unique to each
// subscription of the process.
var consumers atomic.Uint64

// consumerName returns the name of a consumer unique to a subscription,
// made of the broker id, the process id and a number.
func consumerName(
	id string,
) string {
	return fmt.Sprintf("%s-%d-%d", id, os.Getpid(), consumers.Add(1))
}
