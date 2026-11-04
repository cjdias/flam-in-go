package env

import (
	"maps"
	"math"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/joho/godotenv"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config/sources/internal/base"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
)

// newSource returns an env source holding the values of the mappings, read
// from the files and the environment, or the error of the files.
func newSource(
	id string,
	priority int,
	files []string,
	mappings map[string]string,
) (flam.IConfigSource, error) {
	s := &source{
		Source:   base.New(id, priority),
		files:    files,
		mappings: mappings,
	}

	bag, err := s.load()
	if err != nil {
		return nil, newErrRead(creatorCreateMessage, id, files, err)
	}
	s.Swap(bag)
	return s, nil
}

// source is the env [flam.IConfigReloadableSource]. Its configuration does
// not change after its creation; its values change with Reload.
type source struct {
	*base.Source

	files    []string
	mappings map[string]string

	reloadMu sync.Mutex // serializes the reloads
}

// compile time check that *source implements IConfigReloadableSource
var _ flam.IConfigReloadableSource = (*source)(nil)

// Reload reads the files and the environment again. The reloads are
// serialized, so that a slower one never overrides the values of a later
// one.
func (s *source) Reload() (bool, error) {
	s.reloadMu.Lock()
	defer s.reloadMu.Unlock()

	bag, err := s.load()
	if err != nil {
		return false, newErrRead(sourceReloadMessage, s.Id(), s.files, err)
	}
	return s.Swap(bag), nil
}

// sourceReloadMessage is the message of the errors returned by Reload.
const sourceReloadMessage = "unable to reload env config source"

// load returns a new bag of the values of the mappings, the process
// environment overriding the files, or the error of the files.
func (s *source) load() (container.Bag, error) {
	fileEnv := map[string]string{}
	if len(s.files) != 0 {
		var err error
		if fileEnv, err = godotenv.Read(s.files...); err != nil {
			return nil, err
		}
	}

	bag := container.Bag{}
	for _, key := range slices.Sorted(maps.Keys(s.mappings)) {
		text, ok := os.LookupEnv(key)
		if !ok {
			text = fileEnv[key]
		}
		if text == "" {
			continue
		}
		// the paths were checked by the creator: Set cannot fail
		_ = bag.Set(s.mappings[key], sourceValue(text))
	}
	return bag, nil
}

// sourceValue returns the value of the text of a variable: a bool for
// true or false, in any case, an int or a finite float64 when the text
// writes one exactly as Go formats it, so that texts such as 0755, 1.20,
// +1, NaN or Inf stay strings, or else the text.
func sourceValue(
	text string,
) any {
	switch strings.ToLower(text) {
	case "true":
		return true
	case "false":
		return false
	}
	if i, err := strconv.Atoi(text); err == nil && strconv.Itoa(i) == text {
		return i
	}
	if f, err := strconv.ParseFloat(text, 64); err == nil && !math.IsNaN(f) && !math.IsInf(f, 0) &&
		strconv.FormatFloat(f, 'f', -1, 64) == text {
		return f
	}
	return text
}
