package standard

import (
	"context"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// server is the standard [flam.IGrpcServer], serving its services with a
// [grpc.Server] on its address.
type server struct {
	id              string
	address         string
	logger          flam.IGrpcLoggerAdaptor
	grpc            *grpc.Server
	health          *health
	shutdownTimeout time.Duration

	stopped  chan struct{}
	mu       sync.Mutex
	listener net.Listener
	running  bool
	closed   bool
}

// compile time check that *server implements IGrpcServer
var _ flam.IGrpcServer = (*server)(nil)

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

// Run listens on the address of the server and serves its services until
// the context is done or the server is closed, then stops it gracefully,
// letting the calls under way end within the shutdown timeout. A nil
// context never ends.
//
// It fails with ErrRunning when the server runs already, ErrServe when it
// cannot listen on its address or fails to serve, and ErrShutdown when
// the calls under way do not end in time, reporting the error to its
// logger adaptor. Once stopped, by Close or by the end of its context,
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
		s.mu.Lock()
		closed := s.closed
		s.mu.Unlock()
		if closed {
			// stopped by Close, whose shutdown Run waits for
			<-s.stopped
			err = nil
		} else {
			err = s.fail(newErr(ErrServe, serverRunMessage, s.id, err).
				With(ErrorFieldAddress, s.address))
		}
	case <-ctx.Done():
		shutdownErr := s.shutdown()
		<-served // ending once stopped
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

// Close stops the server, as Run does when its context is done, Run
// returning once it is stopped, failing with ErrShutdown when the calls
// under way do not end within the shutdown timeout. It does nothing once
// the server is closed.
func (s *server) Close() error {
	return s.shutdown()
}

// serverRunMessage is the message of the errors returned by Run and Close.
const serverRunMessage = "unable to run standard grpc server"

// listenAndServe listens on the address and serves the services,
// reporting its start to the logger adaptor, while the health service, if
// any, probes the readiness of the application.
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
	if s.health != nil {
		defer s.health.start()()
	}
	return s.grpc.Serve(listener)
}

// shutdown closes the server, once, stopping its grpc.Server gracefully
// within the shutdown timeout, then at once when the calls under way did
// not end in time, a timeout of 0 stopping it at once.
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
	if s.health != nil {
		s.health.stop()
	}
	if s.shutdownTimeout == 0 {
		s.grpc.Stop()
		return nil
	}

	graceful := make(chan struct{})
	go func() {
		defer close(graceful)
		s.grpc.GracefulStop()
	}()
	timer := time.NewTimer(s.shutdownTimeout)
	defer timer.Stop()
	select {
	case <-graceful:
		return nil
	case <-timer.C:
		// the graceful stop ends with the handlers under way, whose calls
		// Stop cancels, but which it does not wait for
		s.grpc.Stop()
		return s.fail(newErr(ErrShutdown, serverRunMessage, s.id, context.DeadlineExceeded))
	}
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
