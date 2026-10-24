package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
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

type User struct {
	Username       string `json:"name"`
	Password       string `json:"password"`
	HashedPassword []byte
	StorageKey     string
	Expires_at     time.Time
	Created_at     time.Time
	Download_count int
	Expired        bool
	Corrupted      bool
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
			expires_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
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

func (f *FileStore) Download(ctx context.Context, user *User) error {
	var userStore User

	query := `
        SELECT username, storage_key, password_hash, expires_at, created_at, downloaded
        FROM files
        WHERE username = $1
    `

	Ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	err := f.db.QueryRowContext(
		Ctx,
		query,
		user.Username,
	).Scan(
		&userStore.Username,
		&userStore.StorageKey,
		&userStore.HashedPassword,
		&userStore.Expires_at,
		&userStore.Created_at,
		&userStore.Download_count,
	)

	if err != nil {
		return fmt.Errorf("Enter correct username")

	}

	err = bcrypt.CompareHashAndPassword(userStore.HashedPassword, []byte(user.Password))
	if err != nil {
		return fmt.Errorf("enter valid username and password")
	}

	if time.Now().After(userStore.Expires_at) {
		userStore.Expired = true
		return fmt.Errorf("file as expired")
	}

	if userStore.Corrupted == true {
		return fmt.Errorf("file as been corrupted")
	}

	if userStore.Download_count > 3 {
		return fmt.Errorf("file as been downloaded")
	}

	return nil
}
