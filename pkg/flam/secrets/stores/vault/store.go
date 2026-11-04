package vault

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"maps"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/secrets"
)

// store is the vault [flam.ISecretStore], reading the secrets of a KV
// version 2 secrets engine with its own HTTP client, and keeping the
// fields of the secrets read for its ttl, while mu guards the cache.
type store struct {
	id        string
	base      string // the URL of the secrets, ending with /data/
	namespace string
	token     string // the token, unless read from tokenFile
	tokenFile string
	timeout   time.Duration
	ttl       time.Duration
	clock     flam.IClock
	transport *http.Transport
	client    *http.Client

	mu    sync.Mutex
	cache map[string]storeEntry // by secret path
}

// storeEntry is the fields of a secret, kept until expires.
type storeEntry struct {
	values  map[string]string
	expires time.Time
}

// compile time check that *store implements ISecretStore
var _ flam.ISecretStore = (*store)(nil)

func (s *store) Id() string {
	return s.id
}

// Close drops the cache and closes the idle connections of the client.
// The store remains usable.
func (s *store) Close() error {
	s.mu.Lock()
	clear(s.cache)
	s.mu.Unlock()

	s.transport.CloseIdleConnections()
	return nil
}

// Get returns the field of the secret of the key, of the path#field form,
// failing with secrets.ErrInvalidKey for a key of another form,
// secrets.ErrNotFound when the secret or the field does not exist, or with
// the errors of the request.
func (s *store) Get(
	ctx context.Context,
	key string,
) (string, error) {
	path, field, ok := strings.Cut(key, "#")
	if !ok || field == "" || !storeValidPath(path) {
		return "", newErrKey(secrets.ErrInvalidKey, storeGetMessage, s.id, key, nil)
	}
	values, err := s.read(ctx, storeGetMessage, key, path)
	if err != nil {
		return "", err
	}
	value, ok := values[field]
	if !ok {
		return "", newErrKey(secrets.ErrNotFound, storeGetMessage, s.id, key, nil)
	}
	return value, nil
}

// GetMap returns the fields of the secret of the path, failing with
// secrets.ErrInvalidKey for an invalid path, secrets.ErrNotFound when the
// secret does not exist, or with the errors of the request.
func (s *store) GetMap(
	ctx context.Context,
	path string,
) (map[string]string, error) {
	if !storeValidPath(path) {
		return nil, newErrKey(secrets.ErrInvalidKey, storeGetMapMessage, s.id, path, nil)
	}
	values, err := s.read(ctx, storeGetMapMessage, path, path)
	if err != nil {
		return nil, err
	}
	return maps.Clone(values), nil
}

// Messages of the errors returned by the store.
const (
	storeGetMessage    = "unable to get vault secret"
	storeGetMapMessage = "unable to get vault secrets"
)

// read returns the fields of the secret of the path, from the cache when
// kept and not expired, or else from the server, keeping them in the cache
// when the store has a ttl. The returned map must not be changed.
func (s *store) read(
	ctx context.Context,
	message string,
	key string,
	path string,
) (map[string]string, error) {
	if values, ok := s.cached(path); ok {
		return values, nil
	}

	token, err := s.readToken()
	if err != nil {
		return nil, newErrKey(ErrToken, message, s.id, key, err)
	}

	if ctx == nil {
		ctx = context.Background()
	}
	if s.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.timeout)
		defer cancel()
	}
	// the method is valid and the URL made of a valid address and escaped
	// elements: the request is always built
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, s.base+storeEscape(path), nil)
	req.Header.Set(HeaderToken, token)
	if s.namespace != "" {
		req.Header.Set(HeaderNamespace, s.namespace)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, newErrKey(ErrRequest, message, s.id, key, err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return nil, newErrKey(secrets.ErrNotFound, message, s.id, key, nil)
	default:
		return nil, newErrKey(ErrStatus, message, s.id, key, nil).
			With(ErrorFieldStatus, resp.StatusCode)
	}

	var body struct {
		Data struct {
			Data map[string]any `json:"data"`
		} `json:"data"`
	}
	decoder := json.NewDecoder(io.LimitReader(resp.Body, maxResponseSize))
	decoder.UseNumber()
	if err := decoder.Decode(&body); err != nil {
		return nil, newErrKey(ErrResponse, message, s.id, key, err)
	}
	if body.Data.Data == nil {
		return nil, newErrKey(secrets.ErrNotFound, message, s.id, key, nil)
	}

	values := make(map[string]string, len(body.Data.Data))
	for field, value := range body.Data.Data {
		values[field] = storeText(value)
	}
	s.keep(path, values)
	return values, nil
}

// cached returns the fields of the secret of the path kept in the cache,
// and whether they are kept and not expired.
func (s *store) cached(
	path string,
) (map[string]string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.cache[path]
	if !ok || !s.clock.Now().Before(entry.expires) {
		return nil, false
	}
	return entry.values, true
}

// keep keeps the fields of the secret of the path in the cache for the ttl
// of the store, unless zero.
func (s *store) keep(
	path string,
	values map[string]string,
) {
	if s.ttl == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cache[path] = storeEntry{values: values, expires: s.clock.Now().Add(s.ttl)}
}

// readToken returns the token of the store, or the content of its token
// file without its surrounding spaces, failing with the error of the
// reading, or with no error for an empty file.
func (s *store) readToken() (string, error) {
	if s.tokenFile == "" {
		return s.token, nil
	}
	data, err := os.ReadFile(s.tokenFile)
	if err != nil {
		return "", err
	}
	token := strings.TrimSpace(string(data))
	if token == "" {
		return "", errEmptyToken
	}
	return token, nil
}

// errEmptyToken is the cause of the ErrToken error of an empty token file.
var errEmptyToken = errors.New("empty token file")

// storeValidPath reports whether the slash separated path is not empty,
// and holds no empty, "." or ".." element.
func storeValidPath(
	path string,
) bool {
	if path == "" {
		return false
	}
	for element := range strings.SplitSeq(path, "/") {
		if element == "" || element == "." || element == ".." {
			return false
		}
	}
	return true
}

// storeEscape returns the slash separated path with each of its elements
// escaped for a URL path.
func storeEscape(
	path string,
) string {
	elements := strings.Split(path, "/")
	for i, element := range elements {
		elements[i] = url.PathEscape(element)
	}
	return strings.Join(elements, "/")
}

// storeText returns a string field as is, and any other JSON value as its
// JSON text.
func storeText(
	value any,
) string {
	if text, ok := value.(string); ok {
		return text
	}
	// a value decoded from JSON always encodes
	data, _ := json.Marshal(value)
	return string(data)
}
