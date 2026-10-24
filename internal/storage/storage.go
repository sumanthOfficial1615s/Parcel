package storage

import (
	"database/sql"

	"github.com/minio/minio-go/v7"
)

type Storage struct {
	Postgres *FileStore
	MinIO    *minio.Client
	Bucket   string
}

func NewStorage(
	db *sql.DB,
	minIO *minio.Client,
	bucket string,
) Storage {
	return Storage{
		Postgres: NewFileStore(db),
		MinIO:    minIO,
		Bucket:   bucket,
	}
}
