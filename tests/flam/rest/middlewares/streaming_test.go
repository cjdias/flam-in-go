package middlewares

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/rest"
	"github.com/cjdias/flam-in-go/pkg/flam/rest/middlewares/log"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// streamingServer returns the server of a router of the service 3
// holding the streaming endpoint 7 of the pattern and the handler, wrapped
// in a log middleware keeping the bodies, so that it writes through its
// tee writer, then in a timeout middleware of 1ms.
func streamingServer(t *testing.T, pattern string, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	logger, _, _ := newLog(t, container.Bag{log.ConfigFieldMaxBody: 1024})
	router, err := rest.NewRouter(3, logger, newTimeout(t, "1ms"))
	testutil.NoError(t, err)
	testutil.NoError(t, router.Handle(flam.RestEndpoint{Id: 7, Pattern: pattern, Streaming: true}, handler))
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	return srv
}

func Test_Streaming(t *testing.T) {
	t.Run("should stream the events through the log and the timeout middlewares", func(t *testing.T) {
		next := make(chan string)
		srv := streamingServer(t, "GET /events", func(w http.ResponseWriter, req *http.Request) {
			stream, err := rest.NewEventStream(w, req)
			if err != nil {
				t.Errorf("NewEventStream() = %v", err)
				return
			}
			for data := range next {
				if stream.Send("", "", data) != nil {
					return
				}
			}
		})

		resp, err := http.Get(srv.URL + "/events")
		testutil.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		testutil.Equal(t, "content type", resp.Header.Get("Content-Type"), "text/event-stream")
		reader := bufio.NewReader(resp.Body)
		for _, data := range []string{"1", "2"} {
			next <- data
			for _, want := range []string{"data: " + data + "\n", "\n"} {
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

	t.Run("should take the connection over through the log and the timeout middlewares", func(t *testing.T) {
		srv := streamingServer(t, "GET /ws", func(w http.ResponseWriter, _ *http.Request) {
			conn, rw, err := http.NewResponseController(w).Hijack()
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
		})

		conn, err := net.Dial("tcp", srv.Listener.Addr().String())
		testutil.NoError(t, err)
		defer func() { _ = conn.Close() }()
		_, err = io.WriteString(conn, "GET /ws HTTP/1.1\r\nHost: test\r\nConnection: Upgrade\r\nUpgrade: echo\r\n\r\n")
		testutil.NoError(t, err)
		reader := bufio.NewReader(conn)
		resp, err := http.ReadResponse(reader, nil)
		testutil.NoError(t, err)
		testutil.Equal(t, "status", resp.StatusCode, http.StatusSwitchingProtocols)
		_, err = io.WriteString(conn, "hello\n")
		testutil.NoError(t, err)
		line, err := reader.ReadString('\n')
		testutil.NoError(t, err)
		testutil.Equal(t, "line", line, "echo hello\n")
	})
}
