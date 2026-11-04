package env

import (
	"context"
	"os"
	"strings"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/secrets"
)

// store is the env [flam.ISecretStore], reading the process environment.
type store struct {
	id     string
	prefix string
}

// compile time check that *store implements ISecretStore
var _ flam.ISecretStore = (*store)(nil)

func (s *store) Id() string {
	return s.id
}

// Close closes nothing: the store holds nothing to release.
func (s *store) Close() error {
	return nil
}

// Get returns the value of the variable of the key, failing with
// secrets.ErrInvalidKey for an empty key, and secrets.ErrNotFound when the
// variable is not set.
func (s *store) Get(
	_ context.Context,
	key string,
) (string, error) {
	if key == "" {
		return "", newErrKey(secrets.ErrInvalidKey, storeGetMessage, s.id, key, "")
	}
	name := s.prefix + storeName(key)
	text, ok := os.LookupEnv(name)
	if !ok {
		return "", newErrKey(secrets.ErrNotFound, storeGetMessage, s.id, key, name)
	}
	return text, nil
}

// GetMap returns the variables whose name starts with the name of the path
// and an underscore, or with the prefix for an empty path, by the rest of
// their name in lower case. It fails with secrets.ErrInvalidKey for an
// empty path of a store without prefix, which would read the whole
// environment, and secrets.ErrNotFound when no variable matches.
func (s *store) GetMap(
	_ context.Context,
	path string,
) (map[string]string, error) {
	start := s.prefix
	if path != "" {
		start += storeName(path) + "_"
	}
	if start == "" {
		return nil, newErrKey(secrets.ErrInvalidKey, storeGetMapMessage, s.id, path, "")
	}

	values := map[string]string{}
	for _, entry := range os.Environ() {
		name, text, _ := strings.Cut(entry, "=")
		if rest, ok := strings.CutPrefix(name, start); ok && rest != "" {
			values[strings.ToLower(rest)] = text
		}
	}
	if len(values) == 0 {
		return nil, newErrKey(secrets.ErrNotFound, storeGetMapMessage, s.id, path, start)
	}
	return values, nil
}

// Messages of the errors returned by the store.
const (
	storeGetMessage    = "unable to get env secret"
	storeGetMapMessage = "unable to get env secrets"
)

// storeName returns the upper snake case name of the key: its letters upper
// cased, an underscore between a lower case letter or a digit and the upper
// case letter following it, and an underscore for any other character.
func storeName(
	key string,
) string {
	var b strings.Builder
	lower := false // whether the last character is a lower case letter or a digit
	for _, r := range key {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteString(strings.ToUpper(string(r)))
			lower = true
		case r >= 'A' && r <= 'Z':
			if lower {
				b.WriteByte('_')
			}
			b.WriteRune(r)
			lower = false
		default:
			b.WriteByte('_')
			lower = false
		}
	}
	return b.String()
}
