package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"
)

const (
	uniqueViolationCode  = "23505"
	activeEmailIndexName = "idx_users_active_email"
	userColumns          = `id, name, email, password_hash, created_at, updated_at, deleted_at`
)

type AuthRepositoryContract interface {
	Create(ctx context.Context, user User) (*User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	ExistsByEmail(ctx context.Context, email string) (bool, error)
}

type AuthRepository struct {
	db *sqlx.DB
}

func NewAuthRepository(db *sqlx.DB) *AuthRepository {
	return &AuthRepository{
		db: db,
	}
}

func (r *AuthRepository) Create(ctx context.Context, user User) (*User, error) {
	query := `
		INSERT INTO users (name, email, password_hash)
		VALUES ($1, $2, $3)
		RETURNING ` + userColumns

	var created User
	err := r.db.GetContext(ctx, &created, query, user.Name, strings.ToLower(user.Email), user.PasswordHash)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolationCode && pgErr.ConstraintName == activeEmailIndexName {
			return nil, ErrUserEmailAlreadyExists
		}
		return nil, fmt.Errorf("create user: %w", err)
	}
	return &created, nil
}

func (r *AuthRepository) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	query := `
		SELECT ` + userColumns + `
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
		LIMIT 1
	`
	var user User
	err := r.db.GetContext(ctx, &user, query, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return &user, nil
}

func (r *AuthRepository) GetByEmail(ctx context.Context, email string) (*User, error) {
	query := `
		SELECT ` + userColumns + `
		FROM users
		WHERE LOWER(email) = LOWER($1) AND deleted_at IS NULL
		LIMIT 1
	`
	var user User
	err := r.db.GetContext(ctx, &user, query, strings.TrimSpace(email))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return &user, nil
}

func (r *AuthRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1 FROM users
			WHERE LOWER(email) = LOWER($1) AND deleted_at IS NULL
		)
	`
	var exists bool
	err := r.db.GetContext(ctx, &exists, query, strings.TrimSpace(email))
	if err != nil {
		return false, fmt.Errorf("check email exists: %w", err)
	}
	return exists, nil
}
