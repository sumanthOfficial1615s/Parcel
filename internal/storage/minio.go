package storage

import (
	"context"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MinIOConfig struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	UseSSL    bool
	Bucket    string
}

func DefaultMinIOConfig() MinIOConfig {
	return MinIOConfig{
		Endpoint:  "localhost:9000",
		AccessKey: "minioadmin",
		SecretKey: "minioadmin",
		UseSSL:    false,
		Bucket:    "uploads",
	}
}

// NewMinIOClient connects to MinIO and makes sure the bucket exists.
func NewMinIOClient(
	ctx context.Context,
	cfg MinIOConfig,
) (*minio.Client, error) {
	client, err := minio.New(
		cfg.Endpoint,
		&minio.Options{
			Creds: credentials.NewStaticV4(
				cfg.AccessKey,
				cfg.SecretKey,
				"",
			),
			Secure: cfg.UseSSL,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("creating minio client: %w", err)
	}

	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("checking bucket: %w", err)
	}

	if !exists {
		if err := client.MakeBucket(
			ctx,
			cfg.Bucket,
			minio.MakeBucketOptions{},
		); err != nil {
			return nil, fmt.Errorf("creating bucket: %w", err)
		}
	}

	return client, nil
}

// MinIOObjectStore is an ObjectStore backed by a single MinIO bucket.
type MinIOObjectStore struct {
	client *minio.Client
	bucket string
}

var _ ObjectStore = (*MinIOObjectStore)(nil)

func NewMinIOObjectStore(client *minio.Client, bucket string) *MinIOObjectStore {
	return &MinIOObjectStore{
		client: client,
		bucket: bucket,
	}
}

func (s *MinIOObjectStore) Put(
	ctx context.Context,
	key string,
	r io.Reader,
) (int64, error) {
	info, err := s.client.PutObject(
		ctx,
		s.bucket,
		key,
		r,
		-1,
		minio.PutObjectOptions{},
	)
	if err != nil {
		return 0, fmt.Errorf("putting object: %w", err)
	}

	return info.Size, nil
}

func (s *MinIOObjectStore) Get(
	ctx context.Context,
	key string,
) (Object, error) {
	object, err := s.client.GetObject(
		ctx,
		s.bucket,
		key,
		minio.GetObjectOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("getting object: %w", err)
	}

	return minioObject{Object: object}, nil
}

func (s *MinIOObjectStore) Remove(ctx context.Context, key string) error {
	if err := s.client.RemoveObject(
		ctx,
		s.bucket,
		key,
		minio.RemoveObjectOptions{},
	); err != nil {
		return fmt.Errorf("removing object: %w", err)
	}

	return nil
}

// minioObject adapts *minio.Object to the storage.Object interface.
type minioObject struct {
	*minio.Object
}

func (o minioObject) Stat() (ObjectInfo, error) {
	info, err := o.Object.Stat()
	if err != nil {
		return ObjectInfo{}, err
	}

	return ObjectInfo{
		Size:         info.Size,
		LastModified: info.LastModified,
	}, nil
}
