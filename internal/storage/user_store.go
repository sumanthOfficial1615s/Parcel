package storage

import (
	"context"
	"database/sql"
	"errors"

	"github.com/lib/pq"
)

type PostgresUserStore struct {
	db *sql.DB
}

var _ UserStore = (*PostgresUserStore)(nil)

func NewPostgresUserStore(db *sql.DB) *PostgresUserStore {
	return &PostgresUserStore{db: db}
}

func (s *PostgresUserStore) Create(ctx context.Context, u *User) error {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	err := s.db.QueryRowContext(
		ctx,
		`INSERT INTO users (name, email, password_hash)
		 VALUES ($1, $2, $3)
		 RETURNING id, role`,
		u.Name, u.Email, u.PasswordHash,
	).Scan(&u.ID, &u.Role)

	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == "23505" {
		return ErrEmailTaken
	}

	return err
}

func (s *PostgresUserStore) GetByEmail(
	ctx context.Context,
	email string,
) (*User, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	var u User

	err := s.db.QueryRowContext(
		ctx,
		`SELECT id, name, email, password_hash, role
		 FROM users
		 WHERE email = $1`,
		email,
	).Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.Role)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}

	return &u, nil
}
