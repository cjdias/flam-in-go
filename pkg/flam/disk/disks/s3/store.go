package s3

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/cjdias/flam-in-go/pkg/flam/disk/disks/internal/objectfs"
)

// store is the [objectfs.Store] of the objects of a bucket, reached with a
// s3 client.
type store struct {
	client *awss3.Client
	bucket string
}

// compile time check that *store implements objectfs.Store
var _ objectfs.Store = (*store)(nil)

func (s *store) Stat(
	ctx context.Context,
	key string,
) (objectfs.Object, error) {
	out, err := s.client.HeadObject(ctx, &awss3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return objectfs.Object{}, storeError(err)
	}
	return objectfs.Object{
		Key:     key,
		Size:    aws.ToInt64(out.ContentLength),
		ModTime: aws.ToTime(out.LastModified),
	}, nil
}

func (s *store) Read(
	ctx context.Context,
	key string,
) ([]byte, error) {
	out, err := s.client.GetObject(ctx, &awss3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, storeError(err)
	}
	defer func() { _ = out.Body.Close() }()
	return io.ReadAll(out.Body)
}

func (s *store) Write(
	ctx context.Context,
	key string,
	data []byte,
) error {
	_, err := s.client.PutObject(ctx, &awss3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(data),
	})
	return err
}

// Delete removes the object of the key, as S3 does not fail for a missing
// one.
func (s *store) Delete(
	ctx context.Context,
	key string,
) error {
	_, err := s.client.DeleteObject(ctx, &awss3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	return err
}

func (s *store) Exists(
	ctx context.Context,
	prefix string,
) (bool, error) {
	out, err := s.client.ListObjectsV2(ctx, &awss3.ListObjectsV2Input{
		Bucket:  aws.String(s.bucket),
		Prefix:  aws.String(prefix),
		MaxKeys: aws.Int32(1),
	})
	if err != nil {
		return false, err
	}
	return len(out.Contents) > 0, nil
}

func (s *store) List(
	ctx context.Context,
	prefix string,
	recursive bool,
) ([]objectfs.Object, error) {
	input := &awss3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(prefix),
	}
	if !recursive {
		input.Delimiter = aws.String("/")
	}

	var objects []objectfs.Object
	pages := awss3.NewListObjectsV2Paginator(s.client, input)
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, object := range page.Contents {
			objects = append(objects, objectfs.Object{
				Key:     aws.ToString(object.Key),
				Size:    aws.ToInt64(object.Size),
				ModTime: aws.ToTime(object.LastModified),
			})
		}
		for _, common := range page.CommonPrefixes {
			objects = append(objects, objectfs.Object{Key: aws.ToString(common.Prefix), Dir: true})
		}
	}
	return objects, nil
}

// Close does nothing, as the client holds nothing to release.
func (s *store) Close() error {
	return nil
}

// storeError returns the error of the SDK, wrapped with fs.ErrNotExist
// when the storage answered with a 404 status, such as for a missing
// object.
func storeError(
	err error,
) error {
	var response *awshttp.ResponseError
	if errors.As(err, &response) && response.HTTPStatusCode() == http.StatusNotFound {
		return fmt.Errorf("%w: %w", fs.ErrNotExist, err)
	}
	return err
}
