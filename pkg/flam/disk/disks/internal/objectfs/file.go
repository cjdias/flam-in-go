package objectfs

import (
	"context"
	"io"
	"io/fs"
	"os"
	"time"

	"github.com/spf13/afero"
	"github.com/spf13/afero/mem"
)

// newFile returns the file of the name, over the object of the key of the
// store, holding the content, modified at the time, for the flags of
// OpenFile, writing its content to the object on Sync and Close when
// flush.
func newFile(
	store Store,
	name string,
	key string,
	data []byte,
	modTime time.Time,
	flag int,
	flush bool,
) afero.File {
	content := mem.CreateFile(name)
	mem.SetMode(content, filePerm)
	handle := mem.NewFileHandle(content)
	_, _ = handle.Write(data) // an open handle does not fail
	mem.SetModTime(content, modTime)
	if flag&(os.O_WRONLY|os.O_RDWR) == 0 {
		handle = mem.NewReadOnlyFileHandle(content)
	} else if flag&os.O_APPEND == 0 {
		_, _ = handle.Seek(0, io.SeekStart) // an open handle does not fail
	}
	return &file{
		File:  handle,
		store: store,
		key:   key,
		flush: flush,
	}
}

// file is a file of a disk, its content held in memory by an
// [afero/mem.File], written to the object of its key by Sync and Close
// when flush.
type file struct {
	*mem.File

	store  Store
	key    string
	flush  bool
	closed bool
}

// compile time check that *file implements afero.File
var _ afero.File = (*file)(nil)

// Close writes the content of the file to its object, when opened for
// writing or created, and closes the file, even when the writing fails.
func (f *file) Close() error {
	if f.closed {
		return pathError("close", f.Name(), fs.ErrClosed)
	}
	f.closed = true
	err := f.write("close")
	_ = f.File.Close() // does not fail
	return err
}

// Sync writes the content of the file to its object, when opened for
// writing or created.
func (f *file) Sync() error {
	if f.closed {
		return pathError("sync", f.Name(), fs.ErrClosed)
	}
	return f.write("sync")
}

// write writes the content of the file to its object, when flush, failing
// with the error of the operation.
func (f *file) write(
	op string,
) error {
	if !f.flush {
		return nil
	}
	data, _ := io.ReadAll(mem.NewReadOnlyFileHandle(f.Data())) // a new handle does not fail
	if err := f.store.Write(context.Background(), f.key, data); err != nil {
		return pathError(op, f.Name(), err)
	}
	return nil
}
