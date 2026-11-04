package etcd

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"time"

	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config/sources/internal/base"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
)

// source is the etcd [flam.IConfigReloadableSource]. Its configuration,
// client and parser do not change after its creation; its values change
// with Reload.
type source struct {
	*base.Source

	client    *clientv3.Client
	key       string // ending with a slash when recursive
	recursive bool
	timeout   time.Duration      // zero when not bound
	parserID  string             // empty for the string values of a tree
	parser    flam.IConfigParser // nil for the string values of a tree

	reloadMu sync.Mutex   // serializes the reloads, guards stamp
	stamp    *sourceStamp // of the values
}

// sourceStamp identifies a version of the keys read: their count and the
// highest of their modification revisions. Any put raises the highest
// revision, the cluster revision growing with every write, and a delete
// alone lowers the count.
type sourceStamp struct {
	count    int
	revision int64
}

// compile time check that *source implements IConfigReloadableSource
var _ flam.IConfigReloadableSource = (*source)(nil)

// Close closes the client. The source must not be reloaded after.
func (s *source) Close() error {
	return s.client.Close()
}

// Reload reads the key, or the tree, again. Keys whose count and highest
// modification revision are those of the values keep them, unchanged,
// without parsing.
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
const sourceReloadMessage = "unable to reload etcd config source"

// load reads and parses the key, or the tree, returning its values and
// stamp, or the error with the message. It returns nil values when the
// stamp is the last one, not nil.
func (s *source) load(
	message string,
	last *sourceStamp,
) (flam.IBag, *sourceStamp, error) {
	ctx := context.Background()
	if s.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.timeout)
		defer cancel()
	}
	var options []clientv3.OpOption
	if s.recursive {
		options = append(options, clientv3.WithPrefix(), clientv3.WithSort(clientv3.SortByKey, clientv3.SortAscend))
	}

	response, err := s.client.Get(ctx, s.key, options...)
	if err != nil {
		return nil, nil, newErrKey(ErrRequest, message, s.Id(), s.key, err)
	}
	if !s.recursive && len(response.Kvs) == 0 {
		return nil, nil, newErrKey(ErrMissingKey, message, s.Id(), s.key, nil)
	}
	stamp := &sourceStamp{count: len(response.Kvs)}
	for _, kv := range response.Kvs {
		stamp.revision = max(stamp.revision, kv.ModRevision)
	}
	if last != nil && *last == *stamp {
		return nil, nil, nil
	}

	var values flam.IBag
	if s.recursive {
		values, err = s.tree(message, response.Kvs)
	} else {
		values, err = s.parse(message, response.Kvs[0])
	}
	if err != nil {
		return nil, nil, err
	}
	return values, stamp, nil
}

// parse returns the value of the pair parsed by the parser, or the error
// with the message.
func (s *source) parse(
	message string,
	kv *mvccpb.KeyValue,
) (flam.IBag, error) {
	values, err := s.parser.Parse(bytes.NewReader(kv.Value))
	if err != nil {
		return nil, newErrKey(ErrParse, message, s.Id(), string(kv.Key), err).
			With(ErrorFieldParser, s.parserID)
	}
	return values, nil
}

// tree returns the bag of the pairs of a recursive source, each stored at
// the path of its key relative to the prefix, its slashes turned into
// dots: the value parsed and merged there, or its string without parser.
// The keys ending with a slash are skipped. The pairs come sorted by key,
// so that a key is stored before the keys under it.
func (s *source) tree(
	message string,
	kvs []*mvccpb.KeyValue,
) (flam.IBag, error) {
	values := container.Bag{}
	for _, kv := range kvs {
		relative := strings.TrimPrefix(string(kv.Key), s.key)
		if relative == "" || strings.HasSuffix(relative, "/") {
			continue
		}
		path := strings.ReplaceAll(relative, "/", container.BagPathSeparator)

		var err error
		if s.parser == nil {
			err = values.Set(path, string(kv.Value))
		} else {
			var document flam.IBag
			if document, err = s.parse(message, kv); err != nil {
				return nil, err
			}
			_, err = values.Merge(path, document)
		}
		if err != nil {
			return nil, newErrKey(ErrTree, message, s.Id(), string(kv.Key), err)
		}
	}
	return values, nil
}
