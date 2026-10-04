package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type File struct {
	ID           string
	UploaderName string
	StorageKey   string
	OriginalName string
	SizeBytes    int64
	ContentHash  string
	PasswordHash string
	ExpiresAt    time.Time
}

type FileStore struct {
	db *sql.DB
}

// NewFileStore builds a FileStore with its database dependency injected
func NewFileStore(db *sql.DB) *FileStore {
	return &FileStore{db: db}
}

// Upload inserts a new file record and fills in the generated ID
func (f *FileStore) Upload(ctx context.Context, file *File) error {
	query := `
		INSERT INTO files(uploader_name, storage_key, original_name, size_bytes, content_hash, password_hash, expires_at)
		VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id
	`
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	err := f.db.QueryRowContext(ctx,
		query,
		file.UploaderName,
		file.StorageKey,
		file.OriginalName,
		file.SizeBytes,
		file.ContentHash,
		file.PasswordHash,
		file.ExpiresAt,
	).Scan(&file.ID)

	if err != nil {
		return fmt.Errorf("inserting file record: %w", err)
	}
	return nil
}
