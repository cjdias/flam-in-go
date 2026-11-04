package broker

import (
	"maps"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// NewMessage returns the [flam.IBrokerMessage] of the id, topic, payload
// and headers, for the broker drivers and for the tests of the handlers.
// The headers are copied, a nil map giving an empty one, while the
// payload is kept as is.
func NewMessage(
	id string,
	topic string,
	payload []byte,
	headers map[string]string,
) flam.IBrokerMessage {
	copied := make(map[string]string, len(headers))
	maps.Copy(copied, headers)
	return &message{id: id, topic: topic, payload: payload, headers: copied}
}

// message is the [flam.IBrokerMessage] returned by [NewMessage].
type message struct {
	id      string
	topic   string
	payload []byte
	headers map[string]string
}

func (m *message) Id() string {
	return m.id
}

func (m *message) Topic() string {
	return m.topic
}

func (m *message) Payload() []byte {
	return m.payload
}

func (m *message) Headers() map[string]string {
	return m.headers
}
