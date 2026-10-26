package storage

import (
	"context"
	"errors"
	"io"
	"time"
)

// File statuses. They mirror the CHECK constraint in the files table.
const (
	StatusPending  = "pending"
	StatusClean    = "clean"
	StatusInfected = "infected"
	StatusExpired  = "expired"
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

// FileStore persists file metadata.
type FileStore interface {
	Create(ctx context.Context, file *File) error
	GetForDownload(ctx context.Context, request *DownloadRequest) (*File, error)

	// GetPendingStorageKey returns the storage key of a file that is
	// still waiting for its virus scan.
	GetPendingStorageKey(ctx context.Context, fileID string) (string, error)
	MarkClean(ctx context.Context, fileID string) error
	MarkInfected(ctx context.Context, fileID string) error

	// GetExpiredStorageKey returns an empty key when the file is not due
	// for deletion yet (or is not eligible).
	GetExpiredStorageKey(ctx context.Context, fileID string) (string, error)
	MarkExpired(ctx context.Context, fileID string) error
}

type ObjectInfo struct {
	Size         int64
	LastModified time.Time
}

// Object is a readable, seekable handle to a stored object.
type Object interface {
	io.Reader
	io.Seeker
	io.Closer

	Stat() (ObjectInfo, error)
}

// ObjectStore stores file contents.
type ObjectStore interface {
	// Put streams r into the store under key and returns the stored size.
	// r may be of unknown length.
	Put(ctx context.Context, key string, r io.Reader) (int64, error)
	Get(ctx context.Context, key string) (Object, error)
	Remove(ctx context.Context, key string) error
}
