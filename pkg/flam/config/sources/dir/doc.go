// Package dir is the configuration source driver of the configuration
// directories: its sources read the files of a directory of an
// [github.com/cjdias/flam-in-go/pkg/flam.IDisk], and of its sub
// directories when recursive, decode each with an
// [github.com/cjdias/flam-in-go/pkg/flam.IConfigParser] and merge them, as
// a [github.com/cjdias/flam-in-go/pkg/flam.IConfigReloadableSource].
//
// A source is configured with:
//   - [ConfigFieldPath]: the path of the directory on the disk, required;
//   - [ConfigFieldRecursive]: whether the files of the sub directories are
//     read too;
//   - [ConfigFieldDisk]: the id of the disk, taken from the
//     [github.com/cjdias/flam-in-go/pkg/flam.IDiskFactory];
//   - [ConfigFieldParser]: the id of the parser of every file, taken from
//     the [github.com/cjdias/flam-in-go/pkg/flam.IConfigParserFactory];
//   - [ConfigFieldPriority]: the priority of the source;
//   - [ConfigFieldReload]: whether the source is reloaded, true by default.
//     When false, the source is created as a
//     [github.com/cjdias/flam-in-go/pkg/flam.IConfigSource] only, not
//     implementing
//     [github.com/cjdias/flam-in-go/pkg/flam.IConfigReloadableSource], so
//     that the reloader of the configuration skips it and its values never
//     change.
//
// A missing field, but the path, takes the value stored at
// [DefaultRecursiveConfigPath], [DefaultDiskConfigPath],
// [DefaultParserConfigPath], [DefaultPriorityConfigPath] or
// [DefaultReloadConfigPath], set by the bundle to not recursive, the local
// disk, no parser, 0 and true.
//
// # Files
//
// The entries of a directory are read in name order, a sub directory being
// read, when recursive, at its place in that order, so that "a.yaml",
// "b/c.yaml" and "d.yaml" are read in that order. The values of the files
// are merged, as by
// [github.com/cjdias/flam-in-go/pkg/flam/container.Bag.Merge], in that
// order: a file overrides the values of the files read before it, so files
// named such as "00-base.yaml" and "10-local.yaml" are layered.
//
// With a parser, every file is parsed with it. Without, a file is parsed
// with the parser named after its extension, in lower case, a .yml file
// using the yaml parser, and a file whose extension names no parser of the
// factory, or with no extension, such as a README, is skipped. As a skipped
// file is part of the listing compared by Reload, a parser added to the
// factory later reads it only once the directory changes.
//
// The paths of the files are joined with slashes, as the disks expect.
//
// The entries whose name starts with a dot are skipped, as hidden files and
// the ..data directories of mounted Kubernetes volumes. A symbolic link to
// a file is read, but one to a directory is not followed, so that a link
// cycle cannot trap the source; the entries that are neither files nor
// directories are skipped.
//
// The disk and the parsers are taken from their factories, and belong to
// them: closing the source closes none. Replace a reloaded file
// atomically, by writing a new file and renaming it over the old one, as a
// file read while written may be empty, a valid document.
//
// # Reload
//
// The directory is read when the source is created, and again by Reload
// when it changed: Reload lists the directory, and reads nothing when the
// files a parser may read, those with an extension or all with a
// configured parser, are the same, in the same order, with the same
// modification time and size as those last parsed. A frequent reload of an
// unchanged directory so costs its listing; any change, such as an added,
// removed, renamed or modified file, reads and parses every file again, as
// they are merged. A change keeping the modification time and size of a
// file, such as a rewrite of the same size within the time resolution of
// the file system, is not seen until the directory changes again.
package dir
