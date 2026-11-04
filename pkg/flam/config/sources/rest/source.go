package rest

import (
	"bytes"
	"context"
	"io"
	"math"
	"mime"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config/sources/internal/base"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/value"
)

// source is the rest [flam.IConfigReloadableSource]. Its configuration,
// client and parser do not change after its creation; its values change
// with Reload.
type source struct {
	*base.Source

	client        flam.IConfigRestClient
	method        string
	uri           string
	headers       http.Header
	timeout       time.Duration             // zero when bound by the client only
	maxSize       int                       // of a response body, in bytes
	path          string                    // empty for the whole response
	timestampPath string                    // empty when not compared
	parserID      string                    // empty when named by media type
	parser        flam.IConfigParser        // nil when named by media type
	parsers       flam.IConfigParserFactory // for the media type parsers

	reloadMu  sync.Mutex // serializes the reloads, guards timestamp
	timestamp any        // of the values, nil without timestamp path
}

// compile time check that *source implements IConfigReloadableSource
var _ flam.IConfigReloadableSource = (*source)(nil)

// Reload requests the configuration again. With a timestamp path, a
// response holding the timestamp of the values keeps them, unchanged.
func (s *source) Reload() (bool, error) {
	s.reloadMu.Lock()
	defer s.reloadMu.Unlock()

	values, timestamp, err := s.load(sourceReloadMessage, s.timestamp)
	if err != nil {
		return false, err
	}
	s.timestamp = timestamp
	if values == nil {
		return false, nil
	}
	return s.Swap(values), nil
}

// sourceReloadMessage is the message of the errors returned by Reload.
const sourceReloadMessage = "unable to reload rest config source"

// load requests and parses the configuration, returning the bag at the
// path of the response and the timestamp at the timestamp path, or the
// error with the message. It returns a nil bag when the timestamp is the
// last one, not nil, of the values already loaded.
func (s *source) load(
	message string,
	last any,
) (flam.IBag, any, error) {
	ctx := context.Background()
	if s.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.timeout)
		defer cancel()
	}
	// the method and URI were checked by the creator: this cannot fail
	req, _ := http.NewRequestWithContext(ctx, s.method, s.uri, nil)
	req.Header = s.headers.Clone()
	// the client sends the Host of the request, not its Host header
	if host := req.Header.Get("Host"); host != "" {
		req.Host = host
	}

	res, err := s.client.Do(req)
	if err == nil && res == nil {
		err = ErrNoResponse
	}
	if err != nil {
		return nil, nil, newErrRequest(message, s.Id(), s.method, s.uri, err)
	}
	defer func() {
		// drained, up to the maximum size, so that the connection is reused
		_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, int64(s.maxSize)))
		_ = res.Body.Close()
	}()

	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, nil, newErrStatus(message, s.Id(), s.method, s.uri, res.StatusCode)
	}

	parserID, parser := s.parserID, s.parser
	if parser == nil {
		contentType := res.Header.Get("Content-Type")
		if parserID = sourceParserOf(contentType); parserID == "" {
			return nil, nil, newErrContentType(message, s.Id(), s.uri, contentType)
		}
		if parser, err = s.parsers.Get(parserID); err != nil {
			return nil, nil, newErrParser(message, s.Id(), parserID, err)
		}
	}

	// one byte more than the maximum tells a larger body
	content, err := io.ReadAll(io.LimitReader(res.Body, int64(min(s.maxSize, math.MaxInt-1))+1))
	if err != nil {
		return nil, nil, newErrParse(message, s.Id(), s.uri, parserID, err)
	}
	if len(content) > s.maxSize {
		return nil, nil, newErrTooLarge(message, s.Id(), s.uri, s.maxSize)
	}
	document, err := parser.Parse(bytes.NewReader(content))
	if err != nil {
		return nil, nil, newErrParse(message, s.Id(), s.uri, parserID, err)
	}

	var timestamp any
	if s.timestampPath != "" {
		if timestamp = document.Get(s.timestampPath); !sourceValidTimestamp(timestamp) {
			return nil, nil, newErrTimestamp(message, s.Id(), s.uri, s.timestampPath)
		}
		if last != nil && value.Equal(timestamp, last) {
			return nil, timestamp, nil
		}
	}

	switch values := document.Get(s.path).(type) {
	case container.Bag:
		if values == nil {
			return container.Bag{}, timestamp, nil
		}
		return values, timestamp, nil
	case map[string]any: // copied, as a parser may share its documents
		if values == nil {
			return container.Bag{}, timestamp, nil
		}
		return container.Bag(values).Clone(), timestamp, nil
	}
	return nil, nil, newErrResponsePath(message, s.Id(), s.uri, s.path)
}

// sourceParserOf returns the id of the parser named after the media type
// of the Content-Type, or "" when it names none.
func sourceParserOf(
	contentType string,
) string {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return ""
	}
	switch {
	case mediaType == "application/json" || strings.HasSuffix(mediaType, "+json"):
		return "json"
	case mediaType == "application/yaml" || mediaType == "application/x-yaml" ||
		mediaType == "text/yaml" || strings.HasSuffix(mediaType, "+yaml"):
		return "yaml"
	}
	return ""
}

// sourceValidTimestamp reports whether the timestamp is a string or a
// number, as decoded by the parsers.
func sourceValidTimestamp(
	timestamp any,
) bool {
	if timestamp == nil {
		return false
	}
	switch reflect.TypeOf(timestamp).Kind() {
	case reflect.String,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	}
	return false
}
