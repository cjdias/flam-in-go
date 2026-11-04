package base

import (
	"io/fs"
	"path"
	"strings"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// Stamp identifies a version of a file: its modification time and size, as
// reported by its disk.
type Stamp struct {
	modTime time.Time
	size    int64
}

// StampOf returns the stamp of the file information. The time is stripped
// of its monotonic reading and location, so that equal instants compare
// equal.
func StampOf(
	info fs.FileInfo,
) Stamp {
	return Stamp{modTime: info.ModTime().UTC().Round(0), size: info.Size()}
}

// ParserOf returns the id of the parser named after the extension of the
// file, in lower case, the yml extension naming the yaml parser, or "" when
// the file has no extension.
func ParserOf(
	file string,
) string {
	ext := strings.ToLower(strings.TrimPrefix(path.Ext(file), "."))
	if alias, ok := parserAliases[ext]; ok {
		return alias
	}
	return ext
}

// parserAliases maps the file extensions to the parser id they stand for,
// when it is not the extension itself.
var parserAliases = map[string]string{
	"yml": "yaml",
}

// ParseFile opens the file of the disk and parses it with the parser,
// unless the stamp of the opened file is the last one, not nil, returning
// the values, nil for an unchanged file, and the stamp, read from the
// opened file, so that it is the one of the content parsed even when the
// file is replaced meanwhile. It returns the error of the disk as readErr,
// or the error of the parser as parseErr.
func ParseFile(
	disk flam.IDisk,
	file string,
	last *Stamp,
	parser flam.IConfigParser,
) (values flam.IBag, stamp Stamp, readErr error, parseErr error) {
	f, err := disk.Open(file)
	if err != nil {
		return nil, Stamp{}, err, nil
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		return nil, Stamp{}, err, nil
	}
	stamp = StampOf(info)
	if last != nil && *last == stamp {
		return nil, stamp, nil, nil
	}
	if values, err = parser.Parse(f); err != nil {
		return nil, Stamp{}, nil, err
	}
	return values, stamp, nil, nil
}
