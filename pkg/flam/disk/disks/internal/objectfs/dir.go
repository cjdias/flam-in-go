package objectfs

import (
	"io"
	"io/fs"
	"os"
	"syscall"

	"github.com/spf13/afero"
)

// dir is a directory of a disk opened for reading, holding the
// information of its entries, listed when opened. Its methods reading or
// writing content fail with [syscall.EISDIR].
type dir struct {
	name    string
	info    os.FileInfo
	entries []os.FileInfo
	offset  int
	closed  bool
}

// compile time check that *dir implements afero.File
var _ afero.File = (*dir)(nil)

func (d *dir) Name() string {
	return d.name
}

func (d *dir) Stat() (os.FileInfo, error) {
	return d.info, nil
}

// Readdir returns the information of the next count entries, or of all
// the remaining ones when count is not positive, as [os.File.Readdir].
func (d *dir) Readdir(
	count int,
) ([]os.FileInfo, error) {
	if d.closed {
		return nil, pathError("readdir", d.name, fs.ErrClosed)
	}
	rest := d.entries[d.offset:]
	if count <= 0 {
		d.offset = len(d.entries)
		return rest, nil
	}
	if len(rest) == 0 {
		return nil, io.EOF
	}
	rest = rest[:min(count, len(rest))]
	d.offset += len(rest)
	return rest, nil
}

// Readdirnames returns the names of the entries Readdir returns.
func (d *dir) Readdirnames(
	n int,
) ([]string, error) {
	entries, err := d.Readdir(n)
	names := make([]string, len(entries))
	for i, entry := range entries {
		names[i] = entry.Name()
	}
	return names, err
}

func (d *dir) Close() error {
	if d.closed {
		return pathError("close", d.name, fs.ErrClosed)
	}
	d.closed = true
	return nil
}

func (d *dir) Sync() error {
	return nil
}

func (d *dir) Read(
	_ []byte,
) (int, error) {
	return 0, pathError("read", d.name, syscall.EISDIR)
}

func (d *dir) ReadAt(
	_ []byte,
	_ int64,
) (int, error) {
	return 0, pathError("read", d.name, syscall.EISDIR)
}

func (d *dir) Seek(
	_ int64,
	_ int,
) (int64, error) {
	return 0, pathError("seek", d.name, syscall.EISDIR)
}

func (d *dir) Write(
	_ []byte,
) (int, error) {
	return 0, pathError("write", d.name, syscall.EISDIR)
}

func (d *dir) WriteAt(
	_ []byte,
	_ int64,
) (int, error) {
	return 0, pathError("write", d.name, syscall.EISDIR)
}

func (d *dir) WriteString(
	_ string,
) (int, error) {
	return 0, pathError("write", d.name, syscall.EISDIR)
}

func (d *dir) Truncate(
	_ int64,
) error {
	return pathError("truncate", d.name, syscall.EISDIR)
}
