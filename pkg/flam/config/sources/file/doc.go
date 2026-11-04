// Package file is the configuration source driver of the configuration
// files: its sources read a file from an
// [github.com/cjdias/flam-in-go/pkg/flam.IDisk] and decode it with an
// [github.com/cjdias/flam-in-go/pkg/flam.IConfigParser], as a
// [github.com/cjdias/flam-in-go/pkg/flam.IConfigReloadableSource].
//
// A source is configured with:
//   - [ConfigFieldPath]: the path of the file on the disk, required;
//   - [ConfigFieldDisk]: the id of the disk, taken from the
//     [github.com/cjdias/flam-in-go/pkg/flam.IDiskFactory];
//   - [ConfigFieldParser]: the id of the parser, taken from the
//     [github.com/cjdias/flam-in-go/pkg/flam.IConfigParserFactory];
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
// [DefaultDiskConfigPath], [DefaultParserConfigPath],
// [DefaultPriorityConfigPath] or [DefaultReloadConfigPath], set by the
// bundle to the local disk, no parser, 0 and true. Without a parser, the
// parser is the one named after the extension of the file, in lower case,
// a .yml file using the yaml parser: "config.json" is read with the json
// parser.
//
// The disk and the parser are taken from their factories when the source is
// created, and belong to them: closing the source closes neither. The file
// is read and parsed when the source is created, and again by Reload when
// it changed: Reload compares the modification time and size of the file
// with those of the file last parsed, and reads nothing when both are the
// same, so that a frequent reload costs a stat of an unchanged file. As a
// consequence, a change keeping both, such as a rewrite of the same size
// within the time resolution of the file system, or a tool restoring the
// modification time, is not seen until the file changes again.
//
// A Reload reading a file while it is written gets what was written so
// far: content the parser refuses fails the Reload, which keeps the
// previous values, but an empty file, a valid document for the json and
// yaml parsers, gives an empty source. Replace a reloaded file atomically,
// by writing a new file and renaming it over the old one.
package file
