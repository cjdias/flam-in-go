package standard

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"slices"
	"time"

	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// httpClient is the standard [flam.IClient], sending the requests with an
// [http.Client] of its own, under the protection of its circuit breaker,
// when it has one, tracing each attempt with its tracer and propagator,
// and measuring it on its metrics, when it has them.
type httpClient struct {
	id            string
	baseURL       *url.URL
	headers       http.Header
	breaker       flam.IClientCircuitBreaker
	retries       int
	retryDelay    time.Duration
	retryMaxDelay time.Duration
	clock         flam.IClock
	http          *http.Client
	tracer        trace.Tracer
	propagator    propagation.TextMapPropagator
	metrics       *clientMetrics
}

// compile time check that *httpClient implements IClient
var _ flam.IClient = (*httpClient)(nil)

func (c *httpClient) Id() string {
	return c.id
}

// Do sends a copy of the request, its URL resolved against the base URL,
// with the default headers it does not set. Each attempt asks the circuit
// breaker first, failing with ErrOpen when refused, then reports its
// outcome to it: a failure for a 5xx response or an error, unless the
// context of the request is done, which is ignored, and a success
// otherwise.
//
// Each attempt runs within a client span of the tracer of the client, when
// it has one, the context of the span being injected into the headers of
// the copy sent, with the propagator, and is measured on the metrics of
// the client, when it has them.
//
// A failed attempt of a request of an idempotent method whose body can be
// sent again, as it has none or its GetBody is set, is retried, up to the
// number of retries, after the retry delay, doubled at each retry up to
// the maximum delay. The response of a failed attempt that is retried is
// discarded, while the last one is returned, whatever its status. The
// errors wrap ErrInvalidRequest, ErrOpen or ErrRequest, and the body of
// the request is closed, even on errors.
func (c *httpClient) Do(
	req *http.Request,
) (*http.Response, error) {
	if req == nil || req.URL == nil {
		if req != nil {
			closeBody(req.Body)
		}
		return nil, newErr(ErrInvalidRequest, clientDoMessage, c.id, nil)
	}
	target := req.URL
	if c.baseURL != nil {
		target = c.baseURL.ResolveReference(req.URL)
	}
	if !target.IsAbs() || target.Host == "" {
		closeBody(req.Body)
		return nil, c.newErrRequest(ErrInvalidRequest, req.Method, target, nil)
	}

	retryable := c.retries > 0 &&
		isIdempotent(req.Method) &&
		(req.Body == nil || req.Body == http.NoBody || req.GetBody != nil)
	for attempt := 0; ; attempt++ {
		if attempt > 0 {
			if err := c.wait(req.Context(), attempt); err != nil {
				return nil, c.newErrRequest(ErrRequest, req.Method, target, err)
			}
		}
		resp, failed, err := c.send(req, target, attempt)
		if !failed || !retryable || attempt == c.retries {
			return resp, err
		}
		if resp != nil {
			discard(resp.Body)
		}
	}
}

// Request creates the request of the method, the URL and the body, under
// the context, then sends it with Do, failing with ErrInvalidRequest when
// it cannot be created, such as for a nil context, an invalid method or an
// invalid URL.
func (c *httpClient) Request(
	ctx context.Context,
	method string,
	rawURL string,
	body io.Reader,
) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return nil, newErr(ErrInvalidRequest, clientDoMessage, c.id, err).
			With(ErrorFieldMethod, method).
			With(ErrorFieldURL, rawURL)
	}
	return c.Do(req)
}

// Close closes the idle connections of the transport of the client.
func (c *httpClient) Close() error {
	c.http.CloseIdleConnections()
	return nil
}

// clientDoMessage is the message of the errors returned by Do and Request.
const clientDoMessage = "unable to do standard client request"

// send sends the attempt of the request to the target URL, within its
// span, measured, and returns its response or error, and whether it
// failed in a way worth a retry, as transmit does.
func (c *httpClient) send(
	req *http.Request,
	target *url.URL,
	attempt int,
) (*http.Response, bool, error) {
	ctx, span := c.startSpan(req, target, attempt)
	start := c.clock.Now()
	resp, failed, err := c.transmit(ctx, req, target, attempt)
	c.observe(span, req.Method, start, resp, err)
	return resp, failed, err
}

// transmit sends a copy of the request, under the context, to the target
// URL, through the circuit breaker, and returns its response or error, and
// whether it failed in a way worth a retry: a 5xx response, or an error
// while the context of the request is not done.
func (c *httpClient) transmit(
	ctx context.Context,
	req *http.Request,
	target *url.URL,
	attempt int,
) (*http.Response, bool, error) {
	out := req.Clone(ctx)
	u := *target
	out.URL = &u
	if out.Header == nil {
		out.Header = http.Header{}
	}
	for name, values := range c.headers {
		if _, ok := out.Header[name]; !ok {
			out.Header[name] = slices.Clone(values)
		}
	}
	if c.propagator != nil {
		c.propagator.Inject(ctx, propagation.HeaderCarrier(out.Header))
	}
	if attempt > 0 && req.GetBody != nil {
		body, err := req.GetBody()
		if err != nil {
			return nil, false, c.newErrRequest(ErrRequest, req.Method, target, err)
		}
		out.Body = body
	}

	var generation uint64
	if c.breaker != nil {
		var ok bool
		if generation, ok = c.breaker.Allow(); !ok {
			closeBody(out.Body)
			return nil, false, c.newErrRequest(ErrOpen, req.Method, target, nil).
				With(ErrorFieldBreaker, c.breaker.Id())
		}
	}

	outcome := flam.ClientOutcomeSuccess
	resp, err := c.http.Do(out)
	switch {
	case err != nil && req.Context().Err() != nil:
		outcome = flam.ClientOutcomeIgnored
	case err != nil, resp.StatusCode >= http.StatusInternalServerError:
		outcome = flam.ClientOutcomeFailure
	}
	if c.breaker != nil {
		c.breaker.Done(generation, outcome)
	}
	if err != nil {
		return nil, outcome == flam.ClientOutcomeFailure, c.newErrRequest(ErrRequest, req.Method, target, err)
	}
	return resp, outcome == flam.ClientOutcomeFailure, nil
}

// wait waits the delay before the retry of the attempt, the retry delay
// doubled at each retry up to the maximum delay, returning the error of
// the context when it is done first.
func (c *httpClient) wait(
	ctx context.Context,
	attempt int,
) error {
	delay := c.retryDelay
	for range attempt - 1 {
		if delay > c.retryMaxDelay/2 {
			delay = c.retryMaxDelay
			break
		}
		delay *= 2
	}

	timer := c.clock.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C():
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// newErrRequest returns the error wrapping the sentinel, joined with the
// cause when not nil, with the ErrorFieldID, ErrorFieldMethod and
// ErrorFieldURL context fields of the request.
func (c *httpClient) newErrRequest(
	sentinel error,
	method string,
	target *url.URL,
	cause error,
) *flam.Error {
	if method == "" {
		method = http.MethodGet
	}
	return newErr(sentinel, clientDoMessage, c.id, cause).
		With(ErrorFieldMethod, method).
		With(ErrorFieldURL, target.Redacted())
}

// isIdempotent reports whether the requests of the method can be sent
// again without changing their effect, an empty method being GET.
func isIdempotent(
	method string,
) bool {
	switch method {
	case "", http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace, http.MethodPut, http.MethodDelete:
		return true
	}
	return false
}

// discard reads a part of the body of a response, so that its connection
// may be reused, then closes it.
func discard(
	body io.ReadCloser,
) {
	_, _ = io.CopyN(io.Discard, body, discardLimit)
	_ = body.Close()
}

// closeBody closes the body of a request not sent, unless nil.
func closeBody(
	body io.ReadCloser,
) {
	if body != nil {
		_ = body.Close()
	}
}
