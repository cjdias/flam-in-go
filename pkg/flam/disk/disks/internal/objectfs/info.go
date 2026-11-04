package objectfs

import (
	"io/fs"
	"time"
)

// fileInfo is the [fs.FileInfo] of a file or a directory of a disk.
type fileInfo struct {
	name    string
	size    int64
	modTime time.Time
	mode    fs.FileMode
}

// compile time check that *fileInfo implements fs.FileInfo
var _ fs.FileInfo = (*fileInfo)(nil)

func (i *fileInfo) Name() string {
	return i.name
}

func (i *fileInfo) Size() int64 {
	return i.size
}

func (i *fileInfo) Mode() fs.FileMode {
	return i.mode
}

func (i *fileInfo) ModTime() time.Time {
	return i.modTime
}

func (i *fileInfo) IsDir() bool {
	return i.mode.IsDir()
}

// Sys returns nil, as the objects have no underlying data source.
func (i *fileInfo) Sys() any {
	return nil
}
