package standard

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// server is the standard [flam.IRestServer], serving its router with an
// [http.Server] on its address.
type server struct {
	id              string
	address         string
	logger          flam.IRestLoggerAdaptor
	http            *http.Server
	shutdownTimeout time.Duration

	stopped  chan struct{}
	mu       sync.Mutex
	listener net.Listener
	running  bool
	closed   bool
}

// compile time check that *server implements IRestServer
var _ flam.IRestServer = (*server)(nil)

func (s *server) Id() string {
	return s.id
}

// Address returns the address the server listens on while it runs, or
// its configured address otherwise.
func (s *server) Address() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener != nil {
		return s.listener.Addr().String()
	}
	return s.address
}

func (s *server) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// Run listens on the address of the server and serves its router until
// the context is done or the server is closed, then shuts it down, letting
// the requests under way end within the shutdown timeout. A nil context
// never ends.
//
// It fails with ErrRunning when the server runs already, ErrServe when it
// cannot listen on its address or fails to serve, and ErrShutdown when
// the requests under way do not end in time, reporting the error to its
// logger adaptor. Once shut down, by Close or by the end of its context,
// the server is closed, and Run returns at once.
func (s *server) Run(
	ctx context.Context,
) error {
	if ctx == nil {
		ctx = context.Background()
	}

	s.mu.Lock()
	switch {
	case s.closed:
		s.mu.Unlock()
		return nil
	case s.running:
		s.mu.Unlock()
		return newErr(ErrRunning, serverRunMessage, s.id, nil)
	}
	s.running = true
	s.mu.Unlock()

	var err error
	served := make(chan struct{})
	go func() {
		defer close(served)
		err = s.listenAndServe()
	}()

	select {
	case <-served:
		if errors.Is(err, http.ErrServerClosed) {
			// closed by Close, whose shutdown Run waits for
			<-s.stopped
			err = nil
		} else {
			err = s.fail(newErr(ErrServe, serverRunMessage, s.id, err).
				With(ErrorFieldAddress, s.address))
		}
	case <-ctx.Done():
		shutdownErr := s.shutdown()
		<-served // ending with http.ErrServerClosed, once shut down
		err = shutdownErr
	}

	s.mu.Lock()
	s.running = false
	s.listener = nil
	s.mu.Unlock()
	if s.logger != nil {
		s.logger.Done(s.id)
	}
	return err
}

// Close shuts the server down, as Run does when its context is done, Run
// returning once the shutdown ends, failing with ErrShutdown when the
// requests under way do not end within the shutdown timeout. It does
// nothing once the server is closed.
func (s *server) Close() error {
	return s.shutdown()
}

// serverRunMessage is the message of the errors returned by Run and Close.
const serverRunMessage = "unable to run standard rest server"

// listenAndServe listens on the address and serves the router, over TLS
// when the server has a certificate, reporting its start to the logger
// adaptor.
func (s *server) listenAndServe() error {
	listener, err := net.Listen("tcp", s.address)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.listener = listener
	s.mu.Unlock()

	if s.logger != nil {
		s.logger.Start(s.id, listener.Addr().String())
	}
	if s.http.TLSConfig != nil {
		return s.http.ServeTLS(listener, "", "")
	}
	return s.http.Serve(listener)
}

// shutdown closes the server, once, shutting its http.Server down within
// the shutdown timeout, then closing it when the requests under way did
// not end in time.
func (s *server) shutdown() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	running := s.running
	s.mu.Unlock()
	defer close(s.stopped)

	if running && s.logger != nil {
		s.logger.Shutdown(s.id)
	}
	ctx, cancel := context.WithTimeout(context.Background(), s.shutdownTimeout)
	defer cancel()
	if err := s.http.Shutdown(ctx); err != nil {
		_ = s.http.Close()
		return s.fail(newErr(ErrShutdown, serverRunMessage, s.id, err))
	}
	return nil
}

// errorLog returns the logger of the errors of the http.Server of the
// server of the id, such as the failed TLS handshakes, reporting each line
// as an error to the logger adaptor, or nil, logging to the standard
// logger, when there is none.
func errorLog(
	id string,
	logger flam.IRestLoggerAdaptor,
) *log.Logger {
	if logger == nil {
		return nil
	}
	return log.New(errorWriter{id: id, logger: logger}, "", 0)
}

// errorWriter reports each line written to it as an error of the server
// of the id to the logger adaptor.
type errorWriter struct {
	id     string
	logger flam.IRestLoggerAdaptor
}

func (w errorWriter) Write(
	line []byte,
) (int, error) {
	w.logger.Error(w.id, errors.New(strings.TrimSuffix(string(line), "\n")))
	return len(line), nil
}

// fail reports the error to the logger adaptor, and returns it.
func (s *server) fail(
	err *flam.Error,
) error {
	if s.logger != nil {
		s.logger.Error(s.id, err)
	}
	return err
}
