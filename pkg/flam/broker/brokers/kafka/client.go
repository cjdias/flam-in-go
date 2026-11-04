package kafka

import (
	kafkago "github.com/segmentio/kafka-go"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// client is the [flam.IBrokerKafkaClient] of kafka-go, used when the
// container provides none.
type client struct{}

// compile time check that client implements IBrokerKafkaClient
var _ flam.IBrokerKafkaClient = client{}

// Writer returns the writer as is, as it connects on its first write.
func (client) Writer(
	writer *kafkago.Writer,
) flam.IBrokerKafkaWriter {
	return writer
}

// Reader returns the reader of kafka-go, which joins its group in the
// background.
func (client) Reader(
	config kafkago.ReaderConfig,
) flam.IBrokerKafkaReader {
	return kafkago.NewReader(config)
}
