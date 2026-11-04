package rest

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrStream is the error wrapped, along with the error of the response
	// writer, by the errors returned from [NewEventStream] and the methods
	// of an [EventStream] when the response cannot be written or flushed.
	ErrStream = errors.New("unable to stream rest events")

	// ErrStreamDone is the error wrapped, along with the error of the
	// context, by the errors returned from the methods of an
	// [EventStream] once the context of its request is done, writing
	// nothing.
	ErrStreamDone = errors.New("rest event stream done")

	// ErrInvalidEvent is the error wrapped by the errors returned from the
	// methods of an [EventStream] given an event name or id holding a line
	// break, an id holding a NUL, a negative retry, or a JSON value that
	// cannot be encoded, along with the error of the encoder, writing
	// nothing, the field being held by [ErrorFieldField].
	ErrInvalidEvent = errors.New("invalid rest event")
)

// NewEventStream starts the Server-Sent Events stream answering the
// request: it writes the 200 status, along with the text/event-stream
// content type, and the headers keeping the proxies from caching or
// buffering the stream, then flushes them. It fails with ErrStream when
// the writer cannot flush, the status being written.
//
// The endpoint streaming the events should be set as Streaming, so that
// the timeout middleware does not buffer its response, and should not be
// wrapped in the middlewares buffering or compressing the responses. A
// writer flushing through a [flam.IRestResponseWriter] cannot tell whether
// the writer it wraps flushes, as with [http.ResponseController].
//
//	func (r *events) stream(w http.ResponseWriter, req *http.Request) {
//		stream, err := rest.NewEventStream(w, req)
//		if err != nil {
//			return
//		}
//		defer stream.Heartbeat(15 * time.Second)()
//		for {
//			select {
//			case <-stream.Done():
//				return
//			case event := <-r.events:
//				if err := stream.SendJSON("update", event.Id, event); err != nil {
//					return
//				}
//			}
//		}
//	}
func NewEventStream(
	w http.ResponseWriter,
	req *http.Request,
) (*EventStream, error) {
	header := w.Header()
	header.Set("Content-Type", contentTypeEventStream)
	header.Set("Cache-Control", "no-cache")
	header.Set("X-Accel-Buffering", "no")
	header.Del("Content-Length")
	w.WriteHeader(http.StatusOK)

	s := &EventStream{
		w:          w,
		controller: http.NewResponseController(w),
		ctx:        req.Context(),
	}
	if err := s.controller.Flush(); err != nil {
		return nil, newErrJoined(ErrStream, eventStreamMessage, err)
	}
	return s, nil
}

// EventStream is the Server-Sent Events stream returned by
// [NewEventStream], writing each event at once, then flushing it, until
// the context of its request is done. It is safe for concurrent use, but
// as a response may not be written once its handler returns, the handler
// should return only once done with it.
type EventStream struct {
	mu         sync.Mutex
	w          http.ResponseWriter
	controller *http.ResponseController
	ctx        context.Context
}

// Done returns the channel closed once the context of the request is
// done, such as when the client went away, after which nothing more is
// written.
func (s *EventStream) Done() <-chan struct{} {
	return s.ctx.Done()
}

// Send writes the event of the name, the id and the data, the name and the
// id being left out when empty, and the data written as one data line per
// line, so that the client receives it as it is.
func (s *EventStream) Send(
	event string,
	id string,
	data string,
) error {
	switch {
	case strings.ContainsAny(event, "\r\n"):
		return newErrInvalidEvent("event", nil)
	case strings.ContainsAny(id, "\r\n\x00"):
		return newErrInvalidEvent("id", nil)
	}

	var b strings.Builder
	if event != "" {
		b.WriteString("event: " + event + "\n")
	}
	if id != "" {
		b.WriteString("id: " + id + "\n")
	}
	for _, line := range lines(data) {
		b.WriteString("data: " + line + "\n")
	}
	b.WriteString("\n")
	return s.write(b.String())
}

// SendJSON writes the event of the name, the id and the JSON encoding of
// the value as its data, as Send does.
func (s *EventStream) SendJSON(
	event string,
	id string,
	v any,
) error {
	data, err := json.Marshal(v)
	if err != nil {
		return newErrInvalidEvent("data", err)
	}
	return s.Send(event, id, string(data))
}

// Comment writes the comment of the text, one comment line per line,
// ignored by the client, but keeping the connection from being closed as
// idle, such as by a proxy.
func (s *EventStream) Comment(
	text string,
) error {
	var b strings.Builder
	for _, line := range lines(text) {
		b.WriteString(":" + line + "\n")
	}
	b.WriteString("\n")
	return s.write(b.String())
}

// Retry writes the time the client should wait for before reconnecting,
// once the connection is lost, in milliseconds.
func (s *EventStream) Retry(
	d time.Duration,
) error {
	if d < 0 {
		return newErrInvalidEvent("retry", nil)
	}
	return s.write("retry: " + strconv.FormatInt(d.Milliseconds(), 10) + "\n\n")
}

// Heartbeat writes an empty comment every interval, in a goroutine of its
// own, until the context of the request is done, a comment cannot be
// written, or the function returned is called, which waits for the
// goroutine to end, so that the handler calls it before returning. An
// interval of 0 or less writes none.
func (s *EventStream) Heartbeat(
	interval time.Duration,
) (stop func()) {
	if interval <= 0 {
		return func() {}
	}

	stopped, ended := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(ended)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-s.ctx.Done():
				return
			case <-stopped:
				return
			case <-ticker.C:
				if s.Comment("") != nil {
					return
				}
			}
		}
	}()

	var once sync.Once
	return func() {
		once.Do(func() { close(stopped) })
		<-ended
	}
}

// contentTypeEventStream is the content type of the responses streamed by
// an EventStream.
const contentTypeEventStream = "text/event-stream"

// eventStreamMessage is the message of the errors returned by
// NewEventStream and the EventStream methods.
const eventStreamMessage = "unable to stream rest event"

// write writes the text, then flushes it, failing with ErrStreamDone once
// the context of the request is done, and with ErrStream when the text
// cannot be written or flushed.
func (s *EventStream) write(
	text string,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ctx.Err(); err != nil {
		return newErrJoined(ErrStreamDone, eventStreamMessage, err)
	}
	if _, err := s.w.Write([]byte(text)); err != nil {
		return newErrJoined(ErrStream, eventStreamMessage, err)
	}
	if err := s.controller.Flush(); err != nil {
		return newErrJoined(ErrStream, eventStreamMessage, err)
	}
	return nil
}

// newErrInvalidEvent returns the error wrapping ErrInvalidEvent, joined
// with the cause when not nil, with the ErrorFieldField context field.
func newErrInvalidEvent(
	field string,
	cause error,
) *flam.Error {
	if cause != nil {
		return newErrJoined(ErrInvalidEvent, eventStreamMessage, cause).
			With(ErrorFieldField, field)
	}
	return flam.NewWrappedError(ErrInvalidEvent, eventStreamMessage).
		With(ErrorFieldField, field)
}

// lines returns the lines of the text, split at each line break, "\r\n",
// "\r" or "\n", as the client splits them.
func lines(
	text string,
) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	return strings.Split(strings.ReplaceAll(text, "\r", "\n"), "\n")
}
