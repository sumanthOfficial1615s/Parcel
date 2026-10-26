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

var (
	ErrFileExpired  = errors.New("file has expired")
	ErrFileNotFound = errors.New("file not found")
	ErrEmailTaken   = errors.New("email already registered")
	ErrUserNotFound = errors.New("user not found")
)

type File struct {
	ID           string
	OwnerID      string
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

// FileSummary is what an owner sees in their file list.
// It deliberately leaves out the password hash and storage key.
type FileSummary struct {
	ID           string     `json:"id"`
	OriginalName string     `json:"filename"`
	SizeBytes    int64      `json:"size"`
	Status       string     `json:"status"`
	ExpiresAt    *time.Time `json:"expires_at"`
	CreatedAt    time.Time  `json:"created_at"`
}

type DownloadRequest struct {
	FileID   string `json:"file_id"`
	Password string `json:"password"`
}

type User struct {
	ID           string
	Name         string
	Email        string
	PasswordHash string
	Role         string // "user" or "admin"
}

// FileStore persists file metadata.
type FileStore interface {
	Create(ctx context.Context, file *File) error
	GetForDownload(ctx context.Context, request *DownloadRequest) (*File, error)

	// ListByOwner returns the files uploaded by the given user, newest first.
	ListByOwner(ctx context.Context, ownerID string) ([]FileSummary, error)

	// DeleteOwned deletes the file if userID owns it or isAdmin is true,
	// and returns its storage key so the caller can remove the object.
	// It returns ErrFileNotFound if the file doesn't exist or isn't the
	// caller's, so the two cases look the same from outside.
	DeleteOwned(ctx context.Context, fileID, userID string, isAdmin bool) (string, error)

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

// UserStore persists user accounts.
type UserStore interface {
	// Create inserts the user and fills in ID and Role.
	// It returns ErrEmailTaken if the email is already registered.
	Create(ctx context.Context, user *User) error

	// GetByEmail returns ErrUserNotFound if there is no such user.
	GetByEmail(ctx context.Context, email string) (*User, error)
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
