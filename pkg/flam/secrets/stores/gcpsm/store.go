package gcpsm

import (
	"context"
	"encoding/json"
	"errors"
	"hash/crc32"
	"io"
	"regexp"
	"strings"
	"sync"
	"time"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/secrets"
)

// store is the gcpsm [flam.ISecretStore], reading the secrets of Google
// Cloud Secret Manager with its own client, and keeping the values read
// for its ttl, while mu guards the cache.
type store struct {
	id      string
	client  *secretmanager.Client
	prefix  string // projects/{project}/secrets/
	version string
	timeout time.Duration
	ttl     time.Duration
	clock   flam.IClock

	mu    sync.Mutex
	cache map[string]storeEntry // by secret id
}

// storeEntry is the value of a secret, kept until expires.
type storeEntry struct {
	value   string
	expires time.Time
}

// compile time check that *store implements ISecretStore
var _ flam.ISecretStore = (*store)(nil)

func (s *store) Id() string {
	return s.id
}

// Close drops the cache and closes the client. The store must not be used
// after.
func (s *store) Close() error {
	s.mu.Lock()
	clear(s.cache)
	s.mu.Unlock()

	return s.client.Close()
}

// Get returns the value of the secret of the key, a secret id, or the
// field of its JSON object for a key of the id#field form, failing with
// secrets.ErrInvalidKey for an invalid id or an empty field,
// secrets.ErrNotFound when the secret, its version or the field does not
// exist, ErrFormat when a field is asked for and the value is not a JSON
// object, or with the errors of the request.
func (s *store) Get(
	ctx context.Context,
	key string,
) (string, error) {
	name, field, hasField := strings.Cut(key, "#")
	if !storeValidID(name) || (hasField && field == "") {
		return "", newErrKey(secrets.ErrInvalidKey, storeGetMessage, s.id, key, nil)
	}
	value, err := s.read(ctx, storeGetMessage, key, name)
	if err != nil || !hasField {
		return value, err
	}
	fields, err := s.fields(storeGetMessage, key, value)
	if err != nil {
		return "", err
	}
	text, ok := fields[field]
	if !ok {
		return "", newErrKey(secrets.ErrNotFound, storeGetMessage, s.id, key, nil)
	}
	return text, nil
}

// GetMap returns the fields of the JSON object of the secret of the id,
// failing with secrets.ErrInvalidKey for an invalid id,
// secrets.ErrNotFound when the secret or its version does not exist,
// ErrFormat when its value is not a JSON object, or with the errors of the
// request.
func (s *store) GetMap(
	ctx context.Context,
	name string,
) (map[string]string, error) {
	if !storeValidID(name) {
		return nil, newErrKey(secrets.ErrInvalidKey, storeGetMapMessage, s.id, name, nil)
	}
	value, err := s.read(ctx, storeGetMapMessage, name, name)
	if err != nil {
		return nil, err
	}
	return s.fields(storeGetMapMessage, name, value)
}

// Messages of the errors returned by the store.
const (
	storeGetMessage    = "unable to get gcpsm secret"
	storeGetMapMessage = "unable to get gcpsm secrets"
)

// read returns the value of the version of the store of the secret of the
// id, from the cache when kept and not expired, or else from the service,
// keeping it in the cache when the store has a ttl.
func (s *store) read(
	ctx context.Context,
	message string,
	key string,
	name string,
) (string, error) {
	if value, ok := s.cached(name); ok {
		return value, nil
	}

	if ctx == nil {
		ctx = context.Background()
	}
	if s.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.timeout)
		defer cancel()
	}
	response, err := s.client.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
		Name: s.prefix + name + "/versions/" + s.version,
	})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return "", newErrKey(secrets.ErrNotFound, message, s.id, key, nil)
		}
		return "", newErrKey(ErrRequest, message, s.id, key, err)
	}

	payload := response.GetPayload()
	data := payload.GetData()
	if payload.DataCrc32C != nil && int64(crc32.Checksum(data, storeCastagnoli)) != *payload.DataCrc32C {
		return "", newErrKey(ErrResponse, message, s.id, key, nil)
	}
	value := string(data)
	s.keep(name, value)
	return value, nil
}

// fields returns the fields of the JSON object of the value, a string field
// as is and any other JSON value as its JSON text, failing with ErrFormat
// when the value is not a JSON object.
func (s *store) fields(
	message string,
	key string,
	value string,
) (map[string]string, error) {
	decoder := json.NewDecoder(strings.NewReader(value))
	decoder.UseNumber()
	var object map[string]any
	if err := decoder.Decode(&object); err != nil || object == nil || storeTrailing(decoder) {
		// the error of the decoding is dropped, as it may quote the value
		return nil, newErrKey(ErrFormat, message, s.id, key, nil)
	}

	fields := make(map[string]string, len(object))
	for field, item := range object {
		if text, ok := item.(string); ok {
			fields[field] = text
			continue
		}
		// a value decoded from JSON always encodes
		data, _ := json.Marshal(item)
		fields[field] = string(data)
	}
	return fields, nil
}

// cached returns the value of the secret of the id kept in the cache, and
// whether it is kept and not expired.
func (s *store) cached(
	name string,
) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.cache[name]
	if !ok || !s.clock.Now().Before(entry.expires) {
		return "", false
	}
	return entry.value, true
}

// keep keeps the value of the secret of the id in the cache for the ttl of
// the store, unless zero.
func (s *store) keep(
	name string,
	value string,
) {
	if s.ttl == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cache[name] = storeEntry{value: value, expires: s.clock.Now().Add(s.ttl)}
}

// storeCastagnoli is the CRC32C table of the checksums of the payloads.
var storeCastagnoli = crc32.MakeTable(crc32.Castagnoli)

// storeIDPattern matches the valid secret ids: 1 to 255 letters, digits,
// hyphens and underscores.
var storeIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,255}$`)

// storeValidID reports whether the secret id is valid.
func storeValidID(
	name string,
) bool {
	return storeIDPattern.MatchString(name)
}

// storeTrailing reports whether the decoder holds more than spaces after
// the decoded document.
func storeTrailing(
	decoder *json.Decoder,
) bool {
	_, err := decoder.Token()
	return !errors.Is(err, io.EOF)
}
