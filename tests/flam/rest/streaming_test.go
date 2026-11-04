package rest

import (
	"bufio"
	"context"
	"errors"
	"io"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/rest"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// streamWriter is a response writer safe for concurrent use, counting its
// writes and its flushes, and failing them with writeErr and flushErr,
// when set.
type streamWriter struct {
	mu       sync.Mutex
	header   http.Header
	status   int
	body     strings.Builder
	writes   int
	flushes  int
	writeErr error
	flushErr error
}

func newStreamWriter() *streamWriter { return &streamWriter{header: http.Header{}} }

func (w *streamWriter) Header() http.Header { return w.header }

func (w *streamWriter) WriteHeader(status int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.status = status
}

func (w *streamWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.writes++
	if w.writeErr != nil {
		return 0, w.writeErr
	}
	return w.body.Write(data)
}

func (w *streamWriter) FlushError() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.flushes++
	if w.flushErr != nil {
		return w.flushErr
	}
	return nil
}

// fail sets the errors of the writes and of the flushes.
func (w *streamWriter) fail(writeErr, flushErr error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.writeErr, w.flushErr = writeErr, flushErr
}

// text returns the body written.
func (w *streamWriter) text() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.body.String()
}

// flushed returns the number of flushes.
func (w *streamWriter) flushed() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.flushes
}

// written returns the number of writes.
func (w *streamWriter) written() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.writes
}

// newStream returns the event stream of a request of the context written
// to a new streamWriter.
func newStream(t *testing.T, ctx context.Context) (*rest.EventStream, *streamWriter) {
	t.Helper()
	w := newStreamWriter()
	stream, err := rest.NewEventStream(w, httptest.NewRequestWithContext(ctx, http.MethodGet, "/events", nil))
	testutil.NoError(t, err)
	return stream, w
}

func Test_EventStream_Declarations(t *testing.T) {
	for name, err := range map[string]error{
		"ErrStream":       rest.ErrStream,
		"ErrStreamDone":   rest.ErrStreamDone,
		"ErrInvalidEvent": rest.ErrInvalidEvent,
	} {
		if err == nil {
			t.Errorf("%s = nil", name)
		}
	}
}

func Test_NewEventStream(t *testing.T) {
	t.Run("should write the status and the headers, then flush them", func(t *testing.T) {
		rec := httptest.NewRecorder()
		rec.Header().Set("Content-Length", "10")
		rec.Header().Set("X-Other", "1")
		stream, err := rest.NewEventStream(rec, httptest.NewRequest(http.MethodGet, "/events", nil))
		testutil.NoError(t, err)
		testutil.Equal(t, "stream", stream != nil, true)
		testutil.Equal(t, "status", rec.Code, http.StatusOK)
		testutil.Equal(t, "flushed", rec.Flushed, true)
		testutil.Equal(t, "header", rec.Header(), http.Header{
			"Content-Type":      {"text/event-stream"},
			"Cache-Control":     {"no-cache"},
			"X-Accel-Buffering": {"no"},
			"X-Other":           {"1"},
		})
	})

	t.Run("should fail when the writer cannot flush", func(t *testing.T) {
		rec := httptest.NewRecorder()
		stream, err := rest.NewEventStream(struct{ http.ResponseWriter }{rec}, httptest.NewRequest(http.MethodGet, "/events", nil))
		testutil.Equal(t, "stream", stream, (*rest.EventStream)(nil))
		testutil.Error(t, err, rest.ErrStream, "unable to stream rest event: unable to stream rest events: feature not supported", map[string]any{})
		if !errors.Is(err, http.ErrNotSupported) {
			t.Errorf("error = %v, want http.ErrNotSupported", err)
		}
		testutil.Equal(t, "status", rec.Code, http.StatusOK)
	})
}

func Test_EventStream_Send(t *testing.T) {
	t.Run("should write the events, then flush them", func(t *testing.T) {
		stream, w := newStream(t, context.Background())
		testutil.NoError(t, stream.Send("update", "7", "a\r\nb\rc\nd"))
		testutil.NoError(t, stream.Send("", "", "only"))
		testutil.NoError(t, stream.Send("", "", ""))
		testutil.Equal(t, "body", w.text(),
			"event: update\nid: 7\ndata: a\ndata: b\ndata: c\ndata: d\n\n"+
				"data: only\n\n"+
				"data: \n\n")
		testutil.Equal(t, "flushes", w.flushed(), 4)
	})

	t.Run("should fail with an invalid event name or id, writing nothing", func(t *testing.T) {
		for name, tc := range map[string]struct {
			event, id, field string
		}{
			"event new line":    {"a\nb", "", "event"},
			"event return":      {"a\rb", "", "event"},
			"id new line":       {"", "1\n", "id"},
			"id carriage":       {"", "1\r", "id"},
			"id nul":            {"", "1\x00", "id"},
			"event before id":   {"\n", "\n", "event"},
			"valid event, id x": {"ok", "\x00", "id"},
		} {
			stream, w := newStream(t, context.Background())
			err := stream.Send(tc.event, tc.id, "data")
			testutil.Error(t, err, rest.ErrInvalidEvent, "unable to stream rest event: invalid rest event",
				map[string]any{rest.ErrorFieldField: tc.field})
			testutil.Equal(t, name+" body", w.text(), "")
		}
	})

	t.Run("should fail once the context is done, writing nothing", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		stream, w := newStream(t, ctx)
		cancel()
		<-stream.Done()
		err := stream.Send("", "", "late")
		testutil.Error(t, err, rest.ErrStreamDone, "unable to stream rest event: rest event stream done: context canceled", map[string]any{})
		if !errors.Is(err, context.Canceled) {
			t.Errorf("error = %v, want context.Canceled", err)
		}
		testutil.Equal(t, "body", w.text(), "")
	})

	t.Run("should fail when the event cannot be written or flushed", func(t *testing.T) {
		stream, w := newStream(t, context.Background())
		w.fail(errBoom, nil)
		testutil.Error(t, stream.Send("", "", "x"), rest.ErrStream, "unable to stream rest event: unable to stream rest events: boom", map[string]any{})

		stream, w = newStream(t, context.Background())
		w.fail(nil, errBoom)
		testutil.Error(t, stream.Send("", "", "x"), rest.ErrStream, "unable to stream rest event: unable to stream rest events: boom", map[string]any{})
		testutil.Equal(t, "body", w.text(), "data: x\n\n")
	})
}

func Test_EventStream_SendJSON(t *testing.T) {
	t.Run("should write the JSON of the value as the data", func(t *testing.T) {
		stream, w := newStream(t, context.Background())
		testutil.NoError(t, stream.SendJSON("user", "3", map[string]any{"name": "a\nb"}))
		testutil.Equal(t, "body", w.text(), "event: user\nid: 3\ndata: {\"name\":\"a\\nb\"}\n\n")
	})

	t.Run("should fail with a value that cannot be encoded, writing nothing", func(t *testing.T) {
		stream, w := newStream(t, context.Background())
		err := stream.SendJSON("", "", math.Inf(1))
		testutil.Error(t, err, rest.ErrInvalidEvent, "unable to stream rest event: invalid rest event: json: unsupported value: +Inf",
			map[string]any{rest.ErrorFieldField: "data"})
		testutil.Equal(t, "body", w.text(), "")
	})
}

func Test_EventStream_Comment(t *testing.T) {
	stream, w := newStream(t, context.Background())
	testutil.NoError(t, stream.Comment(""))
	testutil.NoError(t, stream.Comment(" a\nb"))
	testutil.Equal(t, "body", w.text(), ":\n\n: a\n:b\n\n")
}

func Test_EventStream_Retry(t *testing.T) {
	stream, w := newStream(t, context.Background())
	testutil.NoError(t, stream.Retry(1500*time.Millisecond))
	testutil.NoError(t, stream.Retry(0))
	testutil.Error(t, stream.Retry(-time.Millisecond), rest.ErrInvalidEvent, "unable to stream rest event: invalid rest event",
		map[string]any{rest.ErrorFieldField: "retry"})
	testutil.Equal(t, "body", w.text(), "retry: 1500\n\nretry: 0\n\n")
}

// waitFor waits for the condition to hold, failing the test after 5
// seconds.
func waitFor(t *testing.T, name string, condition func() bool) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); !condition(); {
		if time.Now().After(deadline) {
			t.Fatalf("%s did not happen", name)
		}
		time.Sleep(time.Millisecond)
	}
}

func Test_EventStream_Heartbeat(t *testing.T) {
	t.Run("should write nothing for an interval of 0 or less", func(t *testing.T) {
		stream, w := newStream(t, context.Background())
		stream.Heartbeat(0)()
		stream.Heartbeat(-time.Second)()
		testutil.Equal(t, "body", w.text(), "")
	})

	t.Run("should write empty comments until stopped", func(t *testing.T) {
		stream, w := newStream(t, context.Background())
		stop := stream.Heartbeat(time.Millisecond)
		waitFor(t, "heartbeats", func() bool { return strings.Count(w.text(), ":\n\n") >= 2 })
		testutil.NoError(t, stream.Send("", "", "between"))
		stop()
		stop()
		after := w.text()
		time.Sleep(5 * time.Millisecond)
		testutil.Equal(t, "no more", w.text(), after)
		testutil.Equal(t, "only comments and the event", strings.ReplaceAll(strings.ReplaceAll(after, ":\n\n", ""), "data: between\n\n", ""), "")
	})

	t.Run("should end once the context is done", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		stream, w := newStream(t, ctx)
		stop := stream.Heartbeat(time.Hour)
		cancel()
		// lets the goroutine see the context done before it is stopped
		time.Sleep(20 * time.Millisecond)
		stop()
		testutil.Equal(t, "body", w.text(), "")
	})

	t.Run("should end once a comment cannot be written", func(t *testing.T) {
		stream, w := newStream(t, context.Background())
		w.fail(errBoom, nil)
		stop := stream.Heartbeat(time.Millisecond)
		waitFor(t, "failed write", func() bool { return w.written() >= 1 })
		stop()
		testutil.Equal(t, "body", w.text(), "")
	})
}

func Test_EventStream_Server(t *testing.T) {
	t.Run("should stream the events through a router as they are sent", func(t *testing.T) {
		router, err := rest.NewRouter(1, &middleware{id: "outer"})
		testutil.NoError(t, err)
		next := make(chan string)
		testutil.NoError(t, router.Handle(flam.RestEndpoint{Id: 1, Pattern: "GET /events", Streaming: true},
			http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				stream, err := rest.NewEventStream(w, req)
				if err != nil {
					return
				}
				for data := range next {
					if stream.Send("tick", "", data) != nil {
						return
					}
				}
			})))
		srv := httptest.NewServer(router)
		defer srv.Close()

		resp, err := http.Get(srv.URL + "/events")
		testutil.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		testutil.Equal(t, "content type", resp.Header.Get("Content-Type"), "text/event-stream")
		reader := bufio.NewReader(resp.Body)
		for _, data := range []string{"1", "2"} {
			next <- data
			for _, want := range []string{"event: tick\n", "data: " + data + "\n", "\n"} {
				line, err := reader.ReadString('\n')
				testutil.NoError(t, err)
				testutil.Equal(t, "line", line, want)
			}
		}
		close(next)
		remaining, err := io.ReadAll(reader)
		testutil.NoError(t, err)
		testutil.Equal(t, "remaining", string(remaining), "")
	})
}

// hijack returns the handler taking the connection of its request over,
// through the writer returned by wrap, answering with a 101 status, then
// echoing a line.
func hijack(t *testing.T, wrap func(http.ResponseWriter) http.ResponseWriter) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		conn, rw, err := http.NewResponseController(wrap(w)).Hijack()
		if err != nil {
			t.Errorf("Hijack() = %v", err)
			return
		}
		defer func() { _ = conn.Close() }()
		_, _ = rw.WriteString("HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: echo\r\n\r\n")
		_ = rw.Flush()
		line, _ := rw.ReadString('\n')
		_, _ = rw.WriteString("echo " + line)
		_ = rw.Flush()
	}
}

// upgrade sends an upgrade request to the address, then a line, returning
// the status and the line echoed.
func upgrade(t *testing.T, address string) (int, string) {
	t.Helper()
	conn, err := net.Dial("tcp", address)
	testutil.NoError(t, err)
	defer func() { _ = conn.Close() }()
	_, err = io.WriteString(conn, "GET /ws HTTP/1.1\r\nHost: test\r\nConnection: Upgrade\r\nUpgrade: echo\r\n\r\n")
	testutil.NoError(t, err)
	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, nil)
	testutil.NoError(t, err)
	_, err = io.WriteString(conn, "hello\n")
	testutil.NoError(t, err)
	line, err := reader.ReadString('\n')
	testutil.NoError(t, err)
	return resp.StatusCode, line
}

func Test_ResponseWriter_Hijack(t *testing.T) {
	t.Run("should take the connection over through the writer", func(t *testing.T) {
		srv := httptest.NewServer(hijack(t, func(w http.ResponseWriter) http.ResponseWriter {
			return rest.NewResponseWriter(w)
		}))
		defer srv.Close()
		status, line := upgrade(t, srv.Listener.Addr().String())
		testutil.Equal(t, "status", status, http.StatusSwitchingProtocols)
		testutil.Equal(t, "line", line, "echo hello\n")
	})

	t.Run("should take the connection over through a router", func(t *testing.T) {
		router, err := rest.NewRouter(1, &middleware{id: "outer"})
		testutil.NoError(t, err)
		testutil.NoError(t, router.Handle(flam.RestEndpoint{Pattern: "GET /ws", Streaming: true},
			hijack(t, func(w http.ResponseWriter) http.ResponseWriter { return w })))
		srv := httptest.NewServer(router)
		defer srv.Close()
		status, line := upgrade(t, srv.Listener.Addr().String())
		testutil.Equal(t, "status", status, http.StatusSwitchingProtocols)
		testutil.Equal(t, "line", line, "echo hello\n")
	})
}
