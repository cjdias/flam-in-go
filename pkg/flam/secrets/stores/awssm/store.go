package awssm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/secrets"
)

// store is the awssm [flam.ISecretStore], reading the secrets of AWS
// Secrets Manager with its own client, and keeping the values read for its
// ttl, while mu guards the cache.
type store struct {
	id        string
	client    *secretsmanager.Client
	transport *http.Transport
	stage     string
	timeout   time.Duration
	ttl       time.Duration
	clock     flam.IClock

	mu    sync.Mutex
	cache map[string]storeEntry // by secret name
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

// Close drops the cache and closes the idle connections of the client.
// The store remains usable.
func (s *store) Close() error {
	s.mu.Lock()
	clear(s.cache)
	s.mu.Unlock()

	s.transport.CloseIdleConnections()
	return nil
}

// Get returns the value of the secret of the key, a name, or the field of
// its JSON object for a key of the name#field form, failing with
// secrets.ErrInvalidKey for an empty name or field, secrets.ErrNotFound
// when the secret or the field does not exist, ErrFormat when a field is
// asked for and the value is not a JSON object, or with the errors of the
// request.
func (s *store) Get(
	ctx context.Context,
	key string,
) (string, error) {
	name, field, hasField := strings.Cut(key, "#")
	if name == "" || (hasField && field == "") {
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

// GetMap returns the fields of the JSON object of the secret of the name,
// failing with secrets.ErrInvalidKey for an empty name or one holding a
// #, secrets.ErrNotFound when the secret does not exist, ErrFormat when
// its value is not a JSON object, or with the errors of the request.
func (s *store) GetMap(
	ctx context.Context,
	name string,
) (map[string]string, error) {
	if name == "" || strings.Contains(name, "#") {
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
	storeGetMessage    = "unable to get awssm secret"
	storeGetMapMessage = "unable to get awssm secrets"
)

// read returns the value of the secret of the name, at the version stage
// of the store, from the cache when kept and not expired, or else from the
// service, keeping it in the cache when the store has a ttl.
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
	output, err := s.client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId:     &name,
		VersionStage: &s.stage,
	})
	if err != nil {
		var notFound *types.ResourceNotFoundException
		if errors.As(err, &notFound) {
			return "", newErrKey(secrets.ErrNotFound, message, s.id, key, nil)
		}
		return "", newErrKey(ErrRequest, message, s.id, key, err)
	}

	var value string
	switch {
	case output.SecretString != nil:
		value = *output.SecretString
	case output.SecretBinary != nil:
		value = string(output.SecretBinary)
	default:
		return "", newErrKey(ErrResponse, message, s.id, key, nil)
	}
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

// cached returns the value of the secret of the name kept in the cache,
// and whether it is kept and not expired.
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

// keep keeps the value of the secret of the name in the cache for the ttl
// of the store, unless zero.
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

// storeTrailing reports whether the decoder holds more than spaces after
// the decoded document.
func storeTrailing(
	decoder *json.Decoder,
) bool {
	_, err := decoder.Token()
	return !errors.Is(err, io.EOF)
}
