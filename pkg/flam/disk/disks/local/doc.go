// Package local is the disk driver of the local disks: the file system of
// the operating system, as an [github.com/spf13/afero.OsFs], optionally
// restricted to a root directory, as an
// [github.com/spf13/afero.BasePathFs], with the [ConfigFieldRoot] field of
// the disk configuration.
package local
