// Package objectfs holds what the disk drivers of the object storages,
// such as s3 and gcs, share: [NewDisk] returns a
// [github.com/cjdias/flam-in-go/pkg/flam.IDisk], an
// [github.com/spf13/afero.Fs], over the objects of a bucket, reached
// through the [Store] of the driver.
//
// The paths of the disk are slash separated, relative to its root, which
// is the prefix of the disk inside the bucket: the file of path "a/b.txt"
// is the object of key prefix + "a/b.txt". The paths are cleaned, and
// cannot escape the root.
//
// Object storages have no directories: a directory exists when an object
// has a key under it, such as the empty object of key "a/", a directory
// marker written by Mkdir and MkdirAll, or "a/b.txt". The parents of a
// file or a directory need not exist to create it, and exist once it is.
// The root always exists.
//
// A file is read whole, in memory, when opened, unless truncated, and
// written whole, as the object of its key, by the Sync and Close methods
// of the file opened for writing or created. The objects have no modes,
// owners nor times other than their modification time: the files have
// the mode 0644, the directories 0755, and Chmod, Chown and Chtimes fail
// with [errors.ErrUnsupported].
package objectfs
