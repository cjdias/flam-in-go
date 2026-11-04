package nats

import (
	"context"

	natsgo "github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/broker/brokers/internal/base"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
)

// newCreator returns the [flam.IBrokerCreator] of the NATS brokers,
// reading the defaults of the missing fields from the configuration. It is
// the provider registered by the bundle returned from [NewBundle].
func newCreator(
	config flam.IConfig,
) flam.IBrokerCreator {
	return &creator{config: config}
}

// creator is the [flam.IBrokerCreator] of the configurations with the
// [Driver] driver.
type creator struct {
	config flam.IConfig
}

func (c *creator) Accept(
	bag flam.IBag,
) bool {
	return bag.String(factory.ConfigFieldDriver) == Driver
}

// Create connects to the server of the fields, failing at the first
// invalid one, and, with JetStream and subjects, creates or updates the
// stream with them.
func (c *creator) Create(
	bag flam.IBag,
) (flam.IBroker, error) {
	id := bag.String(factory.ConfigFieldId)
	r := base.NewReader(c.config, bag)

	url := r.String(ConfigFieldURL, DefaultURLConfigPath, defaultURL, true)
	name := r.String(ConfigFieldName, "", id, false)
	token := r.String(ConfigFieldToken, "", "", false)
	user := r.String(ConfigFieldUser, "", "", false)
	password := r.String(ConfigFieldPassword, "", "", false)
	credentials := r.String(ConfigFieldCredentials, "", "", false)
	connectTimeout := r.Duration(ConfigFieldConnectTimeout, DefaultConnectTimeoutConfigPath, defaultConnectTimeout)
	r.Check(ConfigFieldConnectTimeout, connectTimeout > 0)
	reconnectWait := r.Duration(ConfigFieldReconnectWait, DefaultReconnectWaitConfigPath, defaultReconnectWait)
	maxReconnects := r.Int(ConfigFieldMaxReconnects, DefaultMaxReconnectsConfigPath, defaultMaxReconnects, -1)
	b := &natsBroker{
		id:         id,
		queue:      r.String(ConfigFieldQueue, "", "", false),
		jetStream:  r.Bool(ConfigFieldJetStream, DefaultJetStreamConfigPath, defaultJetStream),
		stream:     r.String(ConfigFieldStream, "", "", false),
		durable:    r.String(ConfigFieldDurable, "", id, false),
		ackWait:    r.Duration(ConfigFieldAckWait, DefaultAckWaitConfigPath, defaultAckWait),
		maxDeliver: r.Int(ConfigFieldMaxDeliver, DefaultMaxDeliverConfigPath, defaultMaxDeliver, -1),
		registry:   base.NewRegistry(id),
	}
	subjects := r.Strings(ConfigFieldSubjects, "", nil, false)
	r.Check(ConfigFieldAckWait, b.ackWait > 0)
	r.Check(ConfigFieldStream, b.stream != "" || !b.jetStream)
	if field := r.Invalid(); field != "" {
		return nil, base.NewErrInvalidField(ErrInvalidField, creatorCreateMessage, id, field)
	}

	options := []natsgo.Option{
		natsgo.Name(name),
		natsgo.Timeout(connectTimeout),
		natsgo.ReconnectWait(reconnectWait),
		natsgo.MaxReconnects(maxReconnects),
	}
	if token != "" {
		options = append(options, natsgo.Token(token))
	}
	if user != "" {
		options = append(options, natsgo.UserInfo(user, password))
	}
	if credentials != "" {
		options = append(options, natsgo.UserCredentials(credentials))
	}
	conn, err := natsgo.Connect(url, options...)
	if err != nil {
		return nil, base.NewErr(ErrConnect, creatorCreateMessage, id, err).
			With(ErrorFieldURL, url)
	}
	b.conn = conn
	b.js, _ = jetstream.New(conn) // never fails without options

	if b.jetStream && len(subjects) > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
		defer cancel()
		if _, err := b.js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{Name: b.stream, Subjects: subjects}); err != nil {
			conn.Close()
			return nil, base.NewErr(ErrStream, creatorCreateMessage, id, err).
				With(ErrorFieldStream, b.stream)
		}
	}
	return b, nil
}

// creatorCreateMessage is the message of the errors returned by Create.
const creatorCreateMessage = "unable to create nats broker"
