package flam

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/spf13/afero"
)

type rotatingFileLogWriter struct {
	lock    sync.Locker
	disk    Disk
	file    afero.File
	path    string
	timer   Timer
	year    int
	month   time.Month
	day     int
	current string
}

func newRotatingFileLogWriter(
	disk Disk,
	path string,
	timer Timer,
) (io.Writer, error) {
	writer := &rotatingFileLogWriter{
		lock:  &sync.Mutex{},
		disk:  disk,
		path:  path,
		timer: timer}

	if e := writer.rotate(timer.Now()); e != nil {
		return nil, e
	}

	return writer, nil
}

func (writer *rotatingFileLogWriter) Write(
	output []byte,
) (int, error) {
	writer.lock.Lock()
	defer writer.lock.Unlock()

	if e := writer.checkRotation(); e != nil {
		return 0, e
	}

	return writer.file.Write(output)
}

func (writer *rotatingFileLogWriter) Close() error {
	return writer.file.Close()
}

func (writer *rotatingFileLogWriter) checkRotation() error {
	now := writer.timer.Now()
	if now.Day() != writer.day || now.Month() != writer.month || now.Year() != writer.year {
		return writer.rotate(now)
	}

	return nil
}

func (writer *rotatingFileLogWriter) rotate(
	now time.Time,
) error {
	writer.year = now.Year()
	writer.month = now.Month()
	writer.day = now.Day()
	writer.current = fmt.Sprintf(writer.path, now.Format("2006-01-02"))

	fp, e := writer.disk.OpenFile(writer.current, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if e != nil {
		return e
	}

	if writer.file != nil {
		_ = writer.file.Close()
	}
	writer.file = fp

	return nil
}
