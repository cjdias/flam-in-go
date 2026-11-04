package gcs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"

	"github.com/cjdias/flam-in-go/pkg/flam/disk/disks/internal/objectfs"
)

// store is the [objectfs.Store] of the objects of a bucket, reached with
// the client it closes.
type store struct {
	client *storage.Client
	bucket *storage.BucketHandle
}

// compile time check that *store implements objectfs.Store
var _ objectfs.Store = (*store)(nil)

func (s *store) Stat(
	ctx context.Context,
	key string,
) (objectfs.Object, error) {
	attrs, err := s.bucket.Object(key).Attrs(ctx)
	if err != nil {
		return objectfs.Object{}, storeError(err)
	}
	return objectfs.Object{Key: key, Size: attrs.Size, ModTime: attrs.Updated}, nil
}

func (s *store) Read(
	ctx context.Context,
	key string,
) ([]byte, error) {
	reader, err := s.bucket.Object(key).NewReader(ctx)
	if err != nil {
		return nil, storeError(err)
	}
	defer func() { _ = reader.Close() }()
	return io.ReadAll(reader)
}

// Write uploads the content in a single request.
func (s *store) Write(
	ctx context.Context,
	key string,
	data []byte,
) error {
	writer := s.bucket.Object(key).NewWriter(ctx)
	writer.ChunkSize = 0
	_, _ = writer.Write(data) // its error is returned by Close too
	return writer.Close()
}

// Delete removes the object of the key, a missing one not being an error.
func (s *store) Delete(
	ctx context.Context,
	key string,
) error {
	err := s.bucket.Object(key).Delete(ctx)
	if errors.Is(err, storage.ErrObjectNotExist) {
		return nil
	}
	return err
}

func (s *store) Exists(
	ctx context.Context,
	prefix string,
) (bool, error) {
	_, err := s.bucket.Objects(ctx, &storage.Query{Prefix: prefix}).Next()
	switch {
	case errors.Is(err, iterator.Done):
		return false, nil
	case err != nil:
		return false, err
	}
	return true, nil
}

func (s *store) List(
	ctx context.Context,
	prefix string,
	recursive bool,
) ([]objectfs.Object, error) {
	query := &storage.Query{Prefix: prefix}
	if !recursive {
		query.Delimiter = "/"
	}

	var objects []objectfs.Object
	objectIterator := s.bucket.Objects(ctx, query)
	for {
		attrs, err := objectIterator.Next()
		switch {
		case errors.Is(err, iterator.Done):
			return objects, nil
		case err != nil:
			return nil, err
		case attrs.Prefix != "":
			objects = append(objects, objectfs.Object{Key: attrs.Prefix, Dir: true})
		default:
			objects = append(objects, objectfs.Object{Key: attrs.Name, Size: attrs.Size, ModTime: attrs.Updated})
		}
	}
}

// Close closes the client of the store.
func (s *store) Close() error {
	return s.client.Close()
}

// storeError returns the error of the client, wrapped with fs.ErrNotExist
// for a missing object.
func storeError(
	err error,
) error {
	if errors.Is(err, storage.ErrObjectNotExist) {
		return fmt.Errorf("%w: %w", fs.ErrNotExist, err)
	}
	return err
}
