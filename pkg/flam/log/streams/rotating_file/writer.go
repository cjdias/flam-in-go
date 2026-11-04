package rotatingfile

import (
	"os"
	"strings"
	"time"

	"github.com/spf13/afero"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/log/streams/internal/base"
)

// writer is the writer of a rotating file stream, appending each line to
// the file of the day of its entry. It is not safe for concurrent use: its
// stream serializes its writes and its Close.
type writer struct {
	id     string
	diskID string
	disk   flam.IDisk
	path   string // holding DatePlaceholder

	file  afero.File // of the date below
	year  int
	month time.Month
	day   int
}

// WriteLine writes the line to the file of the day of the timestamp, in
// its location, opening it first when it is not the file of the last
// write. It fails with ErrOpen, writing nothing, when that file cannot be
// opened, and tries again on the next write.
func (w *writer) WriteLine(
	timestamp time.Time,
	line string,
) error {
	if year, month, day := timestamp.Date(); year != w.year || month != w.month || day != w.day {
		if err := w.rotate(writerWriteMessage, timestamp); err != nil {
			return err
		}
	}
	_, err := w.file.WriteString(line)
	return err
}

// Close closes the file of the day.
func (w *writer) Close() error {
	return w.file.Close()
}

// writerWriteMessage is the message of the errors returned by Write.
const writerWriteMessage = "unable to rotate log file"

// rotate opens the file of the day of now, then closes the previous one,
// whose error is dropped, as its entries were written already. It fails
// with ErrOpen and the message, keeping the previous file, when the file
// cannot be opened.
func (w *writer) rotate(
	message string,
	now time.Time,
) error {
	path := strings.Replace(w.path, DatePlaceholder, now.Format(DateFormat), 1)
	file, err := w.disk.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return base.NewErrOpen(sentinels, message, w.id, w.diskID, path, err)
	}
	if w.file != nil {
		_ = w.file.Close() // written already
	}
	w.file = file
	w.year, w.month, w.day = now.Date()
	return nil
}
