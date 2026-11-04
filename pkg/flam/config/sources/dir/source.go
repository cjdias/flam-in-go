package dir

import (
	"errors"
	"io/fs"
	"path"
	"slices"
	"strings"
	"sync"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config/sources/internal/base"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
)

// source is the dir [flam.IConfigReloadableSource]. Its configuration,
// disk and parser do not change after its creation; its values change with
// Reload.
type source struct {
	*base.Source

	diskID    string
	disk      flam.IDisk
	path      string
	recursive bool
	parserID  string                    // empty when inferred per file
	parser    flam.IConfigParser        // nil when inferred per file
	parsers   flam.IConfigParserFactory // for the inferred parsers

	reloadMu sync.Mutex      // serializes the reloads, guards snapshot
	snapshot *sourceSnapshot // of the files the values were parsed from
}

// sourceSnapshot is the list of the files the values of a source were
// parsed from, in reading order, with their stamps.
type sourceSnapshot struct {
	files []sourceFile
}

// sourceFile is a file of a directory listing, with its stamp.
type sourceFile struct {
	path  string
	stamp base.Stamp
}

// compile time check that *source implements IConfigReloadableSource
var _ flam.IConfigReloadableSource = (*source)(nil)

// Reload reads, parses and merges the files again, unless the list of the
// files and the modification time and size of each are those of the files
// the values were parsed from: an unchanged directory is listed, but no
// file is read, and reports no change.
func (s *source) Reload() (bool, error) {
	s.reloadMu.Lock()
	defer s.reloadMu.Unlock()

	values, snapshot, err := s.load(sourceReloadMessage, s.snapshot)
	if err != nil || values == nil {
		return false, err
	}
	s.snapshot = snapshot
	return s.Swap(values), nil
}

// sourceReloadMessage is the message of the errors returned by Reload.
const sourceReloadMessage = "unable to reload dir config source"

// load lists the files of the directory and, unless the listing is the one
// of the last snapshot, reads, parses and merges them, returning their
// values and snapshot, or nil values for an unchanged directory, or the
// error with the message. The stamp of a parsed file is read from the
// opened file, so that it is the one of the content parsed, even when the
// file is replaced meanwhile.
func (s *source) load(
	message string,
	last *sourceSnapshot,
) (flam.IBag, *sourceSnapshot, error) {
	info, err := s.disk.Stat(s.path)
	if err != nil {
		return nil, nil, newErrRead(message, s.Id(), s.diskID, s.path, err)
	}
	if !info.IsDir() {
		return nil, nil, newErrRead(message, s.Id(), s.diskID, s.path, ErrNotDirectory)
	}

	var files []sourceFile
	if err := s.collect(message, s.path, &files); err != nil {
		return nil, nil, err
	}
	if last != nil && slices.Equal(last.files, files) {
		return nil, nil, nil
	}

	var values flam.IBag = container.Bag{}
	for i, file := range files {
		parserID, parser := s.parserID, s.parser
		if parser == nil {
			parserID = base.ParserOf(file.path)
			// only an unknown parser skips the file: another failure, such
			// as a closed factory, must not empty the source
			parser, err = s.parsers.Get(parserID)
			if errors.Is(err, factory.ErrUnknownID) {
				continue
			}
			if err != nil {
				return nil, nil, newErrParser(message, s.Id(), parserID, err)
			}
		}

		fileValues, stamp, err := s.parse(message, file.path, parserID, parser)
		if err != nil {
			return nil, nil, err
		}
		files[i].stamp = stamp
		if values, err = values.Merge("", fileValues); err != nil {
			return nil, nil, newErrMerge(message, s.Id(), file.path, err)
		}
	}
	return values, &sourceSnapshot{files: files}, nil
}

// collect appends to files the files of the directory a parser may read,
// with their stamps, in name order, reading the sub directories at their
// place when recursive, and skipping the hidden entries, the links to
// directories, the entries that are neither files nor directories and,
// without a configured parser, the files with no extension.
func (s *source) collect(
	message string,
	dir string,
	files *[]sourceFile,
) error {
	entries, err := s.readDir(dir)
	if err != nil {
		return newErrRead(message, s.Id(), s.diskID, dir, err)
	}

	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		entryPath := path.Join(dir, entry.Name())
		if entry.Mode()&fs.ModeSymlink != 0 {
			if entry, err = s.disk.Stat(entryPath); err != nil {
				return newErrRead(message, s.Id(), s.diskID, entryPath, err)
			}
			if entry.IsDir() {
				continue // links to directories are not followed
			}
		}
		switch {
		case entry.IsDir():
			if s.recursive {
				if err := s.collect(message, entryPath, files); err != nil {
					return err
				}
			}
		case entry.Mode().IsRegular() && (s.parser != nil || base.ParserOf(entryPath) != ""):
			*files = append(*files, sourceFile{path: entryPath, stamp: base.StampOf(entry)})
		}
	}
	return nil
}

// readDir returns the entries of the directory sorted by name.
func (s *source) readDir(
	dir string,
) ([]fs.FileInfo, error) {
	d, err := s.disk.Open(dir)
	if err != nil {
		return nil, err
	}
	defer func() { _ = d.Close() }()

	entries, err := d.Readdir(-1)
	if err != nil {
		return nil, err
	}
	slices.SortFunc(entries, func(a, b fs.FileInfo) int {
		return strings.Compare(a.Name(), b.Name())
	})
	return entries, nil
}

// parse reads and parses the file with the parser, returning its values
// and the stamp of the opened file.
func (s *source) parse(
	message string,
	file string,
	parserID string,
	parser flam.IConfigParser,
) (flam.IBag, base.Stamp, error) {
	values, stamp, readErr, parseErr := base.ParseFile(s.disk, file, nil, parser)
	switch {
	case readErr != nil:
		return nil, base.Stamp{}, newErrRead(message, s.Id(), s.diskID, file, readErr)
	case parseErr != nil:
		return nil, base.Stamp{}, newErrParse(message, s.Id(), file, parserID, parseErr)
	}
	return values, stamp, nil
}
