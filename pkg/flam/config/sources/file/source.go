package file

import (
	"sync"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config/sources/internal/base"
)

// source is the file [flam.IConfigReloadableSource]. Its configuration,
// disk and parser do not change after its creation; its values change with
// Reload.
type source struct {
	*base.Source

	diskID   string
	disk     flam.IDisk
	path     string
	parserID string
	parser   flam.IConfigParser

	reloadMu sync.Mutex  // serializes the reloads, guards stamp
	stamp    *base.Stamp // of the file the values were parsed from
}

// compile time check that *source implements IConfigReloadableSource
var _ flam.IConfigReloadableSource = (*source)(nil)

// Reload reads and parses the file again, unless its modification time and
// size are those of the file the values were parsed from: an unchanged file
// is not read, and reports no change.
func (s *source) Reload() (bool, error) {
	s.reloadMu.Lock()
	defer s.reloadMu.Unlock()

	values, stamp, err := s.load(sourceReloadMessage, s.stamp)
	if err != nil || values == nil {
		return false, err
	}
	s.stamp = stamp
	return s.Swap(values), nil
}

// sourceReloadMessage is the message of the errors returned by Reload.
const sourceReloadMessage = "unable to reload file config source"

// load opens the file and, unless its stamp is the last one, parses it,
// returning its values and stamp, or nil values for an unchanged file, or
// the error with the message. The stamp is read from the opened file, so
// that it is the one of the content parsed, even when the file is replaced
// meanwhile.
func (s *source) load(
	message string,
	last *base.Stamp,
) (flam.IBag, *base.Stamp, error) {
	values, stamp, readErr, parseErr := base.ParseFile(s.disk, s.path, last, s.parser)
	switch {
	case readErr != nil:
		return nil, nil, newErrRead(message, s.Id(), s.diskID, s.path, readErr)
	case parseErr != nil:
		return nil, nil, newErrParse(message, s.Id(), s.path, s.parserID, parseErr)
	case values == nil:
		return nil, nil, nil
	}
	return values, &stamp, nil
}
