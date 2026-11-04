package file

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path"
	"strings"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/secrets"
)

// store is the file [flam.ISecretStore], reading the files of its
// directory through an [os.Root] opened on each call.
type store struct {
	id   string
	dir  string
	trim bool
}

// compile time check that *store implements ISecretStore
var _ flam.ISecretStore = (*store)(nil)

func (s *store) Id() string {
	return s.id
}

// Close closes nothing: the directory is only open during a call.
func (s *store) Close() error {
	return nil
}

// Get returns the content of the file of the key, failing with
// secrets.ErrInvalidKey for a key naming no file of the directory,
// secrets.ErrNotFound when the file, or the directory, does not exist, and
// ErrRead when it cannot be read.
func (s *store) Get(
	_ context.Context,
	key string,
) (string, error) {
	name := storeName(key)
	if name == "." || !fs.ValidPath(name) {
		return "", newErrKey(secrets.ErrInvalidKey, storeGetMessage, s.id, key, nil)
	}

	root, err := os.OpenRoot(s.dir)
	if err != nil {
		return "", s.fail(storeGetMessage, key, err)
	}
	defer func() { _ = root.Close() }()

	data, err := root.ReadFile(name)
	if err != nil {
		return "", s.fail(storeGetMessage, key, err)
	}
	return s.value(data), nil
}

// GetMap returns the content of the regular files of the directory of the
// key, the path of the contract, or of the store directory for an empty
// key, by their name, but those whose name starts with a dot, and those
// that cannot be opened. It fails with secrets.ErrInvalidKey for a key
// naming no directory of the store directory, secrets.ErrNotFound when the
// directory does not exist, and ErrRead when it, or one of its files,
// cannot be read.
func (s *store) GetMap(
	_ context.Context,
	key string,
) (map[string]string, error) {
	dir := "."
	if key != "" {
		dir = storeName(key)
	}
	if (key != "" && dir == ".") || !fs.ValidPath(dir) {
		return nil, newErrKey(secrets.ErrInvalidKey, storeGetMapMessage, s.id, key, nil)
	}

	root, err := os.OpenRoot(s.dir)
	if err != nil {
		return nil, s.fail(storeGetMapMessage, key, err)
	}
	defer func() { _ = root.Close() }()

	fsys := root.FS()
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, s.fail(storeGetMapMessage, key, err)
	}
	values := map[string]string{}
	for _, entry := range entries {
		name := entry.Name()
		file := path.Join(dir, name)
		if strings.HasPrefix(name, ".") {
			continue
		}
		if info, err := fs.Stat(fsys, file); err != nil || !info.Mode().IsRegular() {
			continue
		}
		data, err := fs.ReadFile(fsys, file)
		if err != nil {
			return nil, s.fail(storeGetMapMessage, key, err)
		}
		values[name] = s.value(data)
	}
	return values, nil
}

// Messages of the errors returned by the store.
const (
	storeGetMessage    = "unable to get file secret"
	storeGetMapMessage = "unable to get file secrets"
)

// fail returns the error of the file system error of the key:
// secrets.ErrNotFound for a missing file or directory, and ErrRead joined
// with the error otherwise.
func (s *store) fail(
	message string,
	key string,
	err error,
) error {
	if errors.Is(err, fs.ErrNotExist) {
		return newErrKey(secrets.ErrNotFound, message, s.id, key, nil)
	}
	return newErrKey(ErrRead, message, s.id, key, err)
}

// value returns the value of the content of a file, without its trailing
// newline when the store trims.
func (s *store) value(
	data []byte,
) string {
	text := string(data)
	if s.trim {
		if trimmed, ok := strings.CutSuffix(text, "\n"); ok {
			text = strings.TrimSuffix(trimmed, "\r")
		}
	}
	return text
}

// storeName returns the slash separated name of the file of the key, the
// # standing for a slash.
func storeName(
	key string,
) string {
	return strings.ReplaceAll(key, "#", "/")
}
