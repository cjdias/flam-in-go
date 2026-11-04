package outbox

import (
	"encoding/json"

	"gorm.io/gorm"
)

// Add stores the message of the topic, payload and headers, which may be
// nil, in the [Table] table with the transaction, so that the message is
// published by a relay only once the transaction commits, and never when
// it rolls back. It fails with the error wrapping [ErrNilTx] when the
// transaction is nil, [ErrInvalidTopic], holding the topic, when the
// topic is empty, or [ErrAdd], holding the topic, when the message cannot
// be stored, as when the table does not exist.
func Add(
	tx *gorm.DB,
	topic string,
	payload []byte,
	headers map[string]string,
) error {
	switch {
	case tx == nil:
		return newErr(ErrNilTx, addMessage, nil).
			With(ErrorFieldTopic, topic)
	case topic == "":
		return newErr(ErrInvalidTopic, addMessage, nil).
			With(ErrorFieldTopic, topic)
	}

	row := Row{Topic: topic, Payload: payload}
	if row.Payload == nil {
		row.Payload = []byte{}
	}
	if len(headers) > 0 {
		encoded, _ := json.Marshal(headers) // a map of strings always encodes
		row.Headers = string(encoded)
	}
	if err := tx.Create(&row).Error; err != nil {
		return newErr(ErrAdd, addMessage, err).
			With(ErrorFieldTopic, topic)
	}
	return nil
}

// AddJSON stores the message of the JSON encoding of the value, as [Add]
// does, failing with the error wrapping [ErrEncode], holding the topic,
// when the value cannot be encoded, or the error of Add.
func AddJSON(
	tx *gorm.DB,
	topic string,
	value any,
	headers map[string]string,
) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return newErr(ErrEncode, addMessage, err).
			With(ErrorFieldTopic, topic)
	}
	return Add(tx, topic, payload, headers)
}

// addMessage is the message of the errors returned by Add and AddJSON.
const addMessage = "unable to add outbox message"
