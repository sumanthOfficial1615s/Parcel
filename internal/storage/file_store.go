package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var ErrFileExpired = errors.New("file has expired")

type File struct {
	ID           string
	UploaderName string
	StorageKey   string
	OriginalName string
	SizeBytes    int64
	ContentHash  string
	PasswordHash string
	Status       string
	ExpiresAt    *time.Time
	CreatedAt    time.Time
}

type DownloadRequest struct {
	FileID   string `json:"file_id"`
	Password string `json:"password"`
}

type FileStore struct {
	db *sql.DB
}

func NewFileStore(db *sql.DB) *FileStore {
	return &FileStore{
		db: db,
	}
}

func (f *FileStore) Upload(ctx context.Context, file *File) error {
	query := `
		INSERT INTO files (
			uploader_name,
			storage_key,
			original_name,
			size_bytes,
			content_hash,
			password_hash,
			status,
			expires_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, 'clean', $7)
		RETURNING id
	`

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	err := f.db.QueryRowContext(
		ctx,
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

func (f *FileStore) GetForDownload(
	ctx context.Context,
	request *DownloadRequest,
) (*File, error) {
	var file File

	query := `
		SELECT
			id,
			uploader_name,
			storage_key,
			original_name,
			size_bytes,
			content_hash,
			password_hash,
			status,
			expires_at,
			created_at
		FROM files
		WHERE id = $1
	`

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	err := f.db.QueryRowContext(
		ctx,
		query,
		request.FileID,
	).Scan(
		&file.ID,
		&file.UploaderName,
		&file.StorageKey,
		&file.OriginalName,
		&file.SizeBytes,
		&file.ContentHash,
		&file.PasswordHash,
		&file.Status,
		&file.ExpiresAt,
		&file.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("file not found")
		}

		return nil, fmt.Errorf("getting file: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword(
		[]byte(file.PasswordHash),
		[]byte(request.Password),
	); err != nil {
		return nil, fmt.Errorf("invalid password")
	}

	if file.Status == "infected" {
		return nil, fmt.Errorf("file is unavailable")
	}

	if file.Status == "expired" {
		return &file, ErrFileExpired
	}

	if file.ExpiresAt != nil && time.Now().After(*file.ExpiresAt) {
		_, err := f.db.ExecContext(
			ctx,
			`UPDATE files SET status = 'expired' WHERE id = $1`,
			file.ID,
		)
		if err != nil {
			return nil, fmt.Errorf("updating expired file: %w", err)
		}

		file.Status = "expired"

		return &file, ErrFileExpired
	}

	return &file, nil
}
