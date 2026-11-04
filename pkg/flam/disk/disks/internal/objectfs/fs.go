package objectfs

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/afero"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// Object is an object of a bucket, as returned by the [Store] methods, or
// a directory, the common prefix of the keys of objects, in the listings.
type Object struct {
	// Key is the whole key of the object, or the common prefix, ending
	// with "/", of a directory.
	Key string

	// Size is the size of the object, in bytes, 0 for a directory.
	Size int64

	// ModTime is the time the object was last modified, the zero time
	// for a directory.
	ModTime time.Time

	// Dir tells whether the entry is a directory of a listing.
	Dir bool
}

// Store is the access to the objects of a bucket a disk of [NewDisk] is
// over, implemented by the disk drivers. Its methods fail with an error
// wrapping [fs.ErrNotExist] when the object they read does not exist.
type Store interface {
	// Stat returns the object of the key.
	Stat(ctx context.Context, key string) (Object, error)

	// Read returns the whole content of the object of the key.
	Read(ctx context.Context, key string) ([]byte, error)

	// Write creates, or replaces, the object of the key with the content.
	Write(ctx context.Context, key string, data []byte) error

	// Delete removes the object of the key, if any: a missing object is
	// not an error.
	Delete(ctx context.Context, key string) error

	// Exists reports whether an object has a key with the prefix.
	Exists(ctx context.Context, prefix string) (bool, error)

	// List returns the objects whose key has the prefix, in any order.
	// Unless recursive, the keys holding a "/" after the
	// prefix are listed once per directory, as the Dir entry of their
	// common prefix, up to that "/".
	List(ctx context.Context, prefix string, recursive bool) ([]Object, error)

	// Close releases the resources of the store, once the disk is
	// closed.
	Close() error
}

// NewDisk returns the [flam.IDisk] of the id over the objects of the
// store whose keys have the prefix, the root of the disk, cleaned of its
// leading and trailing slashes. The Name method of the disk returns name,
// and its Close method, as an [io.Closer], closes the store.
func NewDisk(
	id string,
	name string,
	prefix string,
	store Store,
) flam.IDisk {
	prefix = strings.Trim(prefix, "/")
	if prefix != "" {
		prefix += "/"
	}
	return &diskImpl{
		id:     id,
		name:   name,
		prefix: prefix,
		store:  store,
	}
}

// diskImpl is the [flam.IDisk] returned by [NewDisk]. Its prefix is empty
// or ends with a "/".
type diskImpl struct {
	id     string
	name   string
	prefix string
	store  Store
}

// compile time check that *diskImpl implements IDisk
var _ flam.IDisk = (*diskImpl)(nil)

func (d *diskImpl) Id() string {
	return d.id
}

func (d *diskImpl) Name() string {
	return d.name
}

// Close closes the store of the disk.
func (d *diskImpl) Close() error {
	return d.store.Close()
}

func (d *diskImpl) Create(
	name string,
) (afero.File, error) {
	return d.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, filePerm)
}

func (d *diskImpl) Open(
	name string,
) (afero.File, error) {
	return d.OpenFile(name, os.O_RDONLY, 0)
}

// OpenFile opens the file or directory of the name. The flags O_CREATE,
// O_EXCL, O_TRUNC and O_APPEND are honoured, the latter by starting at
// the end of the file, while the perm is ignored. A directory can only be
// opened for reading.
func (d *diskImpl) OpenFile(
	name string,
	flag int,
	_ os.FileMode,
) (afero.File, error) {
	ctx := context.Background()
	rel := clean(name)
	writable := flag&(os.O_WRONLY|os.O_RDWR) != 0

	info, err := d.stat(ctx, rel)
	switch {
	case err == nil && info.IsDir():
		if writable {
			return nil, pathError("open", name, syscall.EISDIR)
		}
		return d.openDir(ctx, name, rel, info)
	case err == nil:
		if flag&os.O_CREATE != 0 && flag&os.O_EXCL != 0 {
			return nil, pathError("open", name, fs.ErrExist)
		}
		if writable && flag&os.O_TRUNC != 0 {
			return newFile(d.store, name, d.prefix+rel, nil, time.Now(), flag, true), nil
		}
		data, err := d.store.Read(ctx, d.prefix+rel)
		if err != nil {
			return nil, pathError("open", name, err)
		}
		return newFile(d.store, name, d.prefix+rel, data, info.ModTime(), flag, writable), nil
	case !errors.Is(err, fs.ErrNotExist):
		return nil, pathError("open", name, err)
	case flag&os.O_CREATE == 0:
		return nil, pathError("open", name, fs.ErrNotExist)
	}
	return newFile(d.store, name, d.prefix+rel, nil, time.Now(), flag, true), nil
}

func (d *diskImpl) Stat(
	name string,
) (os.FileInfo, error) {
	info, err := d.stat(context.Background(), clean(name))
	if err != nil {
		return nil, pathError("stat", name, err)
	}
	return info, nil
}

// Mkdir writes the marker of the directory, failing with [fs.ErrExist]
// when a file or a directory exists at the name.
func (d *diskImpl) Mkdir(
	name string,
	_ os.FileMode,
) error {
	ctx := context.Background()
	rel := clean(name)

	_, err := d.stat(ctx, rel)
	switch {
	case err == nil:
		return pathError("mkdir", name, fs.ErrExist)
	case !errors.Is(err, fs.ErrNotExist):
		return pathError("mkdir", name, err)
	}
	if err := d.store.Write(ctx, d.prefix+rel+"/", nil); err != nil {
		return pathError("mkdir", name, err)
	}
	return nil
}

// MkdirAll writes the marker of the directory unless it exists, failing
// with [syscall.ENOTDIR] when a file exists at the name.
func (d *diskImpl) MkdirAll(
	name string,
	_ os.FileMode,
) error {
	ctx := context.Background()
	rel := clean(name)

	info, err := d.stat(ctx, rel)
	switch {
	case err == nil && info.IsDir():
		return nil
	case err == nil:
		return pathError("mkdir", name, syscall.ENOTDIR)
	case !errors.Is(err, fs.ErrNotExist):
		return pathError("mkdir", name, err)
	}
	if err := d.store.Write(ctx, d.prefix+rel+"/", nil); err != nil {
		return pathError("mkdir", name, err)
	}
	return nil
}

// Remove removes the file, or the empty directory, of the name, failing
// with [syscall.ENOTEMPTY] for a directory holding entries. Removing the
// empty root does nothing.
func (d *diskImpl) Remove(
	name string,
) error {
	ctx := context.Background()
	rel := clean(name)

	info, err := d.stat(ctx, rel)
	if err != nil {
		return pathError("remove", name, err)
	}
	if !info.IsDir() {
		if err := d.store.Delete(ctx, d.prefix+rel); err != nil {
			return pathError("remove", name, err)
		}
		return nil
	}

	entries, err := d.list(ctx, rel)
	switch {
	case err != nil:
		return pathError("remove", name, err)
	case len(entries) > 0:
		return pathError("remove", name, syscall.ENOTEMPTY)
	case rel == "":
		return nil
	}
	if err := d.store.Delete(ctx, d.dirKey(rel)); err != nil {
		return pathError("remove", name, err)
	}
	return nil
}

// RemoveAll removes the file of the name and every object under the
// directory of the name, the whole disk for the root. A missing path is
// not an error.
func (d *diskImpl) RemoveAll(
	name string,
) error {
	ctx := context.Background()
	rel := clean(name)

	if rel != "" {
		if err := d.store.Delete(ctx, d.prefix+rel); err != nil {
			return pathError("removeall", name, err)
		}
	}
	objects, err := d.store.List(ctx, d.dirKey(rel), true)
	if err != nil {
		return pathError("removeall", name, err)
	}
	for _, object := range objects {
		if err := d.store.Delete(ctx, object.Key); err != nil {
			return pathError("removeall", name, err)
		}
	}
	return nil
}

// Rename moves the file, or every object under the directory, of the old
// name to the new one, copying and then deleting each object, replacing
// any object of the new keys. Renaming a path to itself does nothing,
// while renaming the root, to the root, or a directory into itself, fails
// with [fs.ErrInvalid].
func (d *diskImpl) Rename(
	oldname string,
	newname string,
) error {
	ctx := context.Background()
	oldRel, newRel := clean(oldname), clean(newname)
	fail := func(err error) error {
		return &os.LinkError{Op: "rename", Old: oldname, New: newname, Err: err}
	}

	info, err := d.stat(ctx, oldRel)
	switch {
	case err != nil:
		return fail(err)
	case newRel == oldRel:
		return nil
	case oldRel == "" || newRel == "":
		return fail(fs.ErrInvalid)
	}
	if !info.IsDir() {
		if err := d.move(ctx, d.prefix+oldRel, d.prefix+newRel); err != nil {
			return fail(err)
		}
		return nil
	}

	if strings.HasPrefix(newRel, oldRel+"/") {
		return fail(fs.ErrInvalid)
	}
	oldKey, newKey := d.dirKey(oldRel), d.dirKey(newRel)
	objects, err := d.store.List(ctx, oldKey, true)
	if err != nil {
		return fail(err)
	}
	for _, object := range objects {
		if err := d.move(ctx, object.Key, newKey+strings.TrimPrefix(object.Key, oldKey)); err != nil {
			return fail(err)
		}
	}
	return nil
}

// Chmod fails with [errors.ErrUnsupported], as objects have no mode.
func (d *diskImpl) Chmod(
	name string,
	_ os.FileMode,
) error {
	return pathError("chmod", name, errors.ErrUnsupported)
}

// Chown fails with [errors.ErrUnsupported], as objects have no owner.
func (d *diskImpl) Chown(
	name string,
	_ int,
	_ int,
) error {
	return pathError("chown", name, errors.ErrUnsupported)
}

// Chtimes fails with [errors.ErrUnsupported], as the times of the objects
// are set by the storage.
func (d *diskImpl) Chtimes(
	name string,
	_ time.Time,
	_ time.Time,
) error {
	return pathError("chtimes", name, errors.ErrUnsupported)
}

// Modes of the files and directories of the disks.
const (
	filePerm os.FileMode = 0o644
	dirMode              = fs.ModeDir | 0o755
)

// clean returns the path of the name relative to the root of the disk,
// slash separated, without leading nor trailing slash, empty for the
// root. The ".." elements cannot escape the root.
func clean(
	name string,
) string {
	return strings.TrimPrefix(path.Clean("/"+filepath.ToSlash(name)), "/")
}

// pathError returns the error of the operation on the path.
func pathError(
	op string,
	name string,
	err error,
) error {
	return &fs.PathError{Op: op, Path: name, Err: err}
}

// dirKey returns the prefix of the keys of the objects under the
// directory of the relative path.
func (d *diskImpl) dirKey(
	rel string,
) string {
	if rel == "" {
		return d.prefix
	}
	return d.prefix + rel + "/"
}

// stat returns the information of the file, or else of the directory, of
// the relative path, or an error wrapping fs.ErrNotExist when neither
// exists.
func (d *diskImpl) stat(
	ctx context.Context,
	rel string,
) (os.FileInfo, error) {
	if rel == "" {
		return &fileInfo{name: "/", mode: dirMode}, nil
	}

	object, err := d.store.Stat(ctx, d.prefix+rel)
	switch {
	case err == nil:
		return &fileInfo{name: path.Base(rel), size: object.Size, modTime: object.ModTime, mode: filePerm}, nil
	case !errors.Is(err, fs.ErrNotExist):
		return nil, err
	}

	exists, err := d.store.Exists(ctx, d.dirKey(rel))
	switch {
	case err != nil:
		return nil, err
	case !exists:
		return nil, fs.ErrNotExist
	}
	return &fileInfo{name: path.Base(rel), mode: dirMode}, nil
}

// list returns the information of the entries of the directory of the
// relative path, sorted by name, its marker excluded. The marker of a
// sub directory, as listed by some storages, is the entry of the sub
// directory, listed once.
func (d *diskImpl) list(
	ctx context.Context,
	rel string,
) ([]os.FileInfo, error) {
	key := d.dirKey(rel)
	objects, err := d.store.List(ctx, key, false)
	if err != nil {
		return nil, err
	}

	entries := make([]os.FileInfo, 0, len(objects))
	for _, object := range objects {
		name := strings.TrimPrefix(object.Key, key)
		switch {
		case name == "":
			continue // the marker of the directory
		case object.Dir || strings.HasSuffix(name, "/"):
			entries = append(entries, &fileInfo{name: strings.TrimSuffix(name, "/"), mode: dirMode})
		default:
			entries = append(entries, &fileInfo{name: name, size: object.Size, modTime: object.ModTime, mode: filePerm})
		}
	}
	slices.SortFunc(entries, func(a, b os.FileInfo) int {
		return strings.Compare(a.Name(), b.Name())
	})
	return slices.CompactFunc(entries, func(a, b os.FileInfo) bool {
		return a.Name() == b.Name() && a.IsDir() == b.IsDir()
	}), nil
}

// openDir returns the directory of the name and relative path, with the
// information, holding its entries.
func (d *diskImpl) openDir(
	ctx context.Context,
	name string,
	rel string,
	stat os.FileInfo,
) (afero.File, error) {
	entries, err := d.list(ctx, rel)
	if err != nil {
		return nil, pathError("open", name, err)
	}
	return &dir{name: name, info: stat, entries: entries}, nil
}

// move copies the object of the old key to the new one, and deletes the
// old one.
func (d *diskImpl) move(
	ctx context.Context,
	oldKey string,
	newKey string,
) error {
	data, err := d.store.Read(ctx, oldKey)
	if err != nil {
		return err
	}
	if err := d.store.Write(ctx, newKey, data); err != nil {
		return err
	}
	return d.store.Delete(ctx, oldKey)
}
