package auth

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrUserNameRequired        = errors.New("name is required")
	ErrUserEmailRequired       = errors.New("email is required")
	ErrUserEmailInvalid        = errors.New("invalid email format")
	ErrUserPasswordRequired    = errors.New("password is required")
	ErrUserPasswordTooShort    = errors.New("password must be at least 6 characters")
	ErrUserEmailAlreadyExists  = errors.New("email is already registered")
	ErrInvalidCredentials      = errors.New("invalid email or password")
	ErrUserNotFound            = errors.New("user not found")
	ErrUnauthorized            = errors.New("unauthorized")
)

type User struct {
	ID           uuid.UUID  `db:"id"`
	Name         string     `db:"name"`
	Email        string     `db:"email"`
	PasswordHash string     `db:"password_hash"`
	CreatedAt    time.Time  `db:"created_at"`
	UpdatedAt    time.Time  `db:"updated_at"`
	DeletedAt    *time.Time `db:"deleted_at"`
}

type RegisterInput struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type UserResponse struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type AuthResponse struct {
	Token string       `json:"token"`
	User  UserResponse `json:"user"`
}
