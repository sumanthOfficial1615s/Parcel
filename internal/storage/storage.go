package storage

import (
	"context"
	"database/sql"
)

type Storage struct {
	Files interface {
		Upload(ctx context.Context, file *File) error
	}
}

func NewStorage(db *sql.DB) Storage {
	return Storage{
		Files: &FileStore{db: db},
	}
}
