package kafka

import (
	kafkago "github.com/segmentio/kafka-go"
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/broker/brokers/internal/base"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
)

// creatorArgs are the dependencies of the creator, taken from the
// container, the client being optional.
type creatorArgs struct {
	dig.In

	Config flam.IConfig
	Client flam.IBrokerKafkaClient `optional:"true"`
}

// newCreator returns the [flam.IBrokerCreator] of the Kafka brokers,
// reading the defaults of the missing fields from the configuration, and
// using the client of the container, or else kafka-go. It is the provider
// registered by the bundle returned from [NewBundle].
func newCreator(
	args creatorArgs,
) flam.IBrokerCreator {
	c := &creator{config: args.Config, client: args.Client}
	if c.client == nil {
		c.client = client{}
	}
	return c
}

// creator is the [flam.IBrokerCreator] of the configurations with the
// [Driver] driver.
type creator struct {
	config flam.IConfig
	client flam.IBrokerKafkaClient
}

func (c *creator) Accept(
	bag flam.IBag,
) bool {
	return bag.String(factory.ConfigFieldDriver) == Driver
}

// Create returns the broker of the fields, failing at the first invalid
// one, with its writer. It does not reach the cluster.
func (c *creator) Create(
	bag flam.IBag,
) (flam.IBroker, error) {
	id := bag.String(factory.ConfigFieldId)
	r := base.NewReader(c.config, bag)

	brokers := r.Strings(ConfigFieldBrokers, DefaultBrokersConfigPath, []string{defaultBroker}, true)
	groupID := r.String(ConfigFieldGroupID, "", id, true)
	balancer := r.OneOf(ConfigFieldBalancer, DefaultBalancerConfigPath, defaultBalancer, BalancerHash, BalancerRoundRobin, BalancerLeastBytes)
	acks := r.OneOf(ConfigFieldRequiredAcks, DefaultRequiredAcksConfigPath, defaultRequiredAcks, AcksNone, AcksOne, AcksAll)
	async := r.Bool(ConfigFieldAsync, "", false)
	batchSize := r.Int(ConfigFieldBatchSize, DefaultBatchSizeConfigPath, defaultBatchSize, 1)
	batchBytes := r.Int(ConfigFieldBatchBytes, DefaultBatchBytesConfigPath, defaultBatchBytes, 1)
	batchTimeout := r.Duration(ConfigFieldBatchTimeout, DefaultBatchTimeoutConfigPath, defaultBatchTimeout)
	writeTimeout := r.Duration(ConfigFieldWriteTimeout, DefaultWriteTimeoutConfigPath, defaultWriteTimeout)
	maxAttempts := r.Int(ConfigFieldMaxAttempts, DefaultMaxAttemptsConfigPath, defaultMaxAttempts, 1)
	autoCreateTopic := r.Bool(ConfigFieldAutoCreateTopic, "", false)
	minBytes := r.Int(ConfigFieldMinBytes, DefaultMinBytesConfigPath, defaultMinBytes, 1)
	maxBytes := r.Int(ConfigFieldMaxBytes, DefaultMaxBytesConfigPath, defaultMaxBytes, max(minBytes, 1))
	maxWait := r.Duration(ConfigFieldMaxWait, DefaultMaxWaitConfigPath, defaultMaxWait)
	startOffset := r.OneOf(ConfigFieldStartOffset, DefaultStartOffsetConfigPath, defaultStartOffset, OffsetFirst, OffsetLast)
	commitInterval := r.Duration(ConfigFieldCommitInterval, "", 0)
	retryBackoff := r.Duration(ConfigFieldRetryBackoff, DefaultRetryBackoffConfigPath, defaultRetryBackoff)
	r.Check(ConfigFieldRetryBackoff, retryBackoff > 0) // a zero backoff retries a failing handler in a busy loop
	if field := r.Invalid(); field != "" {
		return nil, base.NewErrInvalidField(ErrInvalidField, creatorCreateMessage, id, field)
	}

	writer := &kafkago.Writer{
		Addr:                   kafkago.TCP(brokers...),
		Balancer:               balancers[balancer](),
		RequiredAcks:           requiredAcks[acks],
		Async:                  async,
		BatchSize:              batchSize,
		BatchBytes:             int64(batchBytes),
		BatchTimeout:           batchTimeout,
		WriteTimeout:           writeTimeout,
		MaxAttempts:            maxAttempts,
		AllowAutoTopicCreation: autoCreateTopic,
	}
	return &kafkaBroker{
		id:     id,
		client: c.client,
		writer: c.client.Writer(writer),
		reader: kafkago.ReaderConfig{
			Brokers:        brokers,
			GroupID:        groupID,
			MinBytes:       minBytes,
			MaxBytes:       maxBytes,
			MaxWait:        maxWait,
			StartOffset:    startOffsets[startOffset],
			CommitInterval: commitInterval,
		},
		retryBackoff: retryBackoff,
		registry:     base.NewRegistry(id),
	}, nil
}

// creatorCreateMessage is the message of the errors returned by Create.
const creatorCreateMessage = "unable to create kafka broker"

// balancers make the kafka-go balancers of the balancer names, one per
// writer, as they hold state.
var balancers = map[string]func() kafkago.Balancer{
	BalancerHash:       func() kafkago.Balancer { return &kafkago.Hash{} },
	BalancerRoundRobin: func() kafkago.Balancer { return &kafkago.RoundRobin{} },
	BalancerLeastBytes: func() kafkago.Balancer { return &kafkago.LeastBytes{} },
}

// requiredAcks are the kafka-go acknowledgements of the names.
var requiredAcks = map[string]kafkago.RequiredAcks{
	AcksNone: kafkago.RequireNone,
	AcksOne:  kafkago.RequireOne,
	AcksAll:  kafkago.RequireAll,
}

// startOffsets are the kafka-go offsets of the names.
var startOffsets = map[string]int64{
	OffsetFirst: kafkago.FirstOffset,
	OffsetLast:  kafkago.LastOffset,
}
