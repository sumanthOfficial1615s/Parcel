package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const queryTimeout = 5 * time.Second

type PostgresFileStore struct {
	db *sql.DB
}

var _ FileStore = (*PostgresFileStore)(nil)

func NewPostgresFileStore(db *sql.DB) *PostgresFileStore {
	return &PostgresFileStore{db: db}
}

func (s *PostgresFileStore) Create(ctx context.Context, file *File) error {
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
		VALUES ($1, $2, $3, $4, $5, $6, 'pending', $7)
		RETURNING id
	`

	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	err := s.db.QueryRowContext(
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

	file.Status = StatusPending

	return nil
}

func (s *PostgresFileStore) GetPendingStorageKey(
	ctx context.Context,
	fileID string,
) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	var storageKey string

	err := s.db.QueryRowContext(
		ctx,
		`SELECT storage_key
		 FROM files
		 WHERE id = $1
		   AND status = 'pending'`,
		fileID,
	).Scan(&storageKey)

	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("file not found or already scanned")
	}

	if err != nil {
		return "", fmt.Errorf("getting storage key: %w", err)
	}

	return storageKey, nil
}

func (s *PostgresFileStore) MarkClean(
	ctx context.Context,
	fileID string,
) error {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	_, err := s.db.ExecContext(
		ctx,
		`UPDATE files
		 SET status = 'clean'
		 WHERE id = $1
		   AND status = 'pending'`,
		fileID,
	)
	if err != nil {
		return fmt.Errorf("marking file clean: %w", err)
	}

	return nil
}

func (s *PostgresFileStore) MarkInfected(
	ctx context.Context,
	fileID string,
) error {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	_, err := s.db.ExecContext(
		ctx,
		`UPDATE files
		 SET status = 'infected'
		 WHERE id = $1
		   AND status = 'pending'`,
		fileID,
	)
	if err != nil {
		return fmt.Errorf("marking file infected: %w", err)
	}

	return nil
}

func (s *PostgresFileStore) GetForDownload(
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

	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	err := s.db.QueryRowContext(
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

	if file.Status == StatusInfected {
		return nil, fmt.Errorf("file is unavailable")
	}

	if file.Status == StatusPending {
		return nil, fmt.Errorf("file is still being scanned")
	}

	if file.Status == StatusExpired {
		return &file, ErrFileExpired
	}

	if file.ExpiresAt != nil && !time.Now().Before(*file.ExpiresAt) {
		_, err := s.db.ExecContext(
			ctx,
			`UPDATE files SET status = 'expired' WHERE id = $1`,
			file.ID,
		)
		if err != nil {
			return nil, fmt.Errorf("updating expired file: %w", err)
		}

		file.Status = StatusExpired

		return &file, ErrFileExpired
	}

	return &file, nil
}

func (s *PostgresFileStore) GetExpiredStorageKey(
	ctx context.Context,
	fileID string,
) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	var storageKey string

	err := s.db.QueryRowContext(
		ctx,
		`SELECT storage_key
		 FROM files
		 WHERE id = $1
		   AND expires_at <= NOW()
		   AND status IN ('clean', 'expired')`,
		fileID,
	).Scan(&storageKey)

	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}

	if err != nil {
		return "", fmt.Errorf("getting expired file: %w", err)
	}

	return storageKey, nil
}

func (s *PostgresFileStore) MarkExpired(
	ctx context.Context,
	fileID string,
) error {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	_, err := s.db.ExecContext(
		ctx,
		`UPDATE files
		 SET status = 'expired'
		 WHERE id = $1
		   AND expires_at <= NOW()
		   AND status IN ('clean', 'expired')`,
		fileID,
	)
	if err != nil {
		return fmt.Errorf("marking file expired: %w", err)
	}

	return nil
}
