package consul

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/consul/api"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config/sources/internal/base"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
)

// source is the consul [flam.IConfigReloadableSource]. Its configuration,
// client and parser do not change after its creation; its values change
// with Reload.
type source struct {
	*base.Source

	kv        *api.KV
	transport *http.Transport
	key       string // ending with a slash when recursive
	recursive bool
	timeout   time.Duration      // zero when not bound
	parserID  string             // empty for the string values of a tree
	parser    flam.IConfigParser // nil for the string values of a tree

	reloadMu sync.Mutex // serializes the reloads, guards index
	index    uint64     // the X-Consul-Index of the values, zero when unknown
}

// compile time check that *source implements IConfigReloadableSource
var _ flam.IConfigReloadableSource = (*source)(nil)

// Close closes the idle connections of the client. The source remains
// usable.
func (s *source) Close() error {
	s.transport.CloseIdleConnections()
	return nil
}

// Reload reads the key, or the tree, again. A response whose
// X-Consul-Index is the one of the values keeps them, unchanged, without
// parsing.
func (s *source) Reload() (bool, error) {
	s.reloadMu.Lock()
	defer s.reloadMu.Unlock()

	values, index, err := s.load(sourceReloadMessage, s.index)
	if err != nil {
		return false, err
	}
	s.index = index
	if values == nil {
		return false, nil
	}
	return s.Swap(values), nil
}

// sourceReloadMessage is the message of the errors returned by Reload.
const sourceReloadMessage = "unable to reload consul config source"

// load reads and parses the key, or the tree, returning its values and the
// X-Consul-Index of the response, or the error with the message. It
// returns nil values when the index is the last one, not zero.
func (s *source) load(
	message string,
	last uint64,
) (flam.IBag, uint64, error) {
	ctx := context.Background()
	if s.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.timeout)
		defer cancel()
	}
	query := (&api.QueryOptions{}).WithContext(ctx)

	var pairs api.KVPairs
	var meta *api.QueryMeta
	var err error
	if s.recursive {
		pairs, meta, err = s.kv.List(s.key, query)
	} else {
		var pair *api.KVPair
		if pair, meta, err = s.kv.Get(s.key, query); pair != nil {
			pairs = api.KVPairs{pair}
		}
	}
	if err != nil {
		return nil, 0, newErrKey(ErrRequest, message, s.Id(), s.key, err)
	}
	if !s.recursive && len(pairs) == 0 {
		return nil, 0, newErrKey(ErrMissingKey, message, s.Id(), s.key, nil)
	}
	if last != 0 && meta.LastIndex == last {
		return nil, last, nil
	}

	var values flam.IBag
	if s.recursive {
		values, err = s.tree(message, pairs)
	} else {
		values, err = s.parse(message, pairs[0])
	}
	if err != nil {
		return nil, 0, err
	}
	return values, meta.LastIndex, nil
}

// parse returns the value of the pair parsed by the parser, or the error
// with the message.
func (s *source) parse(
	message string,
	pair *api.KVPair,
) (flam.IBag, error) {
	values, err := s.parser.Parse(bytes.NewReader(pair.Value))
	if err != nil {
		return nil, newErrKey(ErrParse, message, s.Id(), pair.Key, err).
			With(ErrorFieldParser, s.parserID)
	}
	return values, nil
}

// tree returns the bag of the pairs of a recursive source, each stored at
// the path of its key relative to the prefix, its slashes turned into
// dots: the value parsed and merged there, or its string without parser.
// The folders, keys ending with a slash, are skipped. The pairs come
// sorted by key, so that a key is stored before the keys under it.
func (s *source) tree(
	message string,
	pairs api.KVPairs,
) (flam.IBag, error) {
	values := container.Bag{}
	for _, pair := range pairs {
		relative := strings.TrimPrefix(pair.Key, s.key)
		if relative == "" || strings.HasSuffix(relative, "/") {
			continue
		}
		path := strings.ReplaceAll(relative, "/", container.BagPathSeparator)

		var err error
		if s.parser == nil {
			err = values.Set(path, string(pair.Value))
		} else {
			var document flam.IBag
			if document, err = s.parse(message, pair); err != nil {
				return nil, err
			}
			_, err = values.Merge(path, document)
		}
		if err != nil {
			return nil, newErrKey(ErrTree, message, s.Id(), pair.Key, err)
		}
	}
	return values, nil
}
