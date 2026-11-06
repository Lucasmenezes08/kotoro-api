package auth

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type AuthServiceContract interface {
	Register(ctx context.Context, input RegisterInput) (*AuthResponse, error)
	Login(ctx context.Context, input LoginInput) (*AuthResponse, error)
	GetMe(ctx context.Context, userID uuid.UUID) (*UserResponse, error)
}

type AuthService struct {
	repo          AuthRepositoryContract
	tokenProvider TokenProvider
}

func NewAuthService(repo AuthRepositoryContract, tokenProvider TokenProvider) *AuthService {
	return &AuthService{
		repo:          repo,
		tokenProvider: tokenProvider,
	}
}

func (s *AuthService) Register(ctx context.Context, input RegisterInput) (*AuthResponse, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, ErrUserNameRequired
	}

	email := strings.ToLower(strings.TrimSpace(input.Email))
	if email == "" {
		return nil, ErrUserEmailRequired
	}
	if !isValidEmail(email) {
		return nil, ErrUserEmailInvalid
	}

	if input.Password == "" {
		return nil, ErrUserPasswordRequired
	}
	if len(input.Password) < 6 {
		return nil, ErrUserPasswordTooShort
	}

	exists, err := s.repo.ExistsByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("check email: %w", err)
	}
	if exists {
		return nil, ErrUserEmailAlreadyExists
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	created, err := s.repo.Create(ctx, User{
		Name:         name,
		Email:        email,
		PasswordHash: string(hash),
	})
	if err != nil {
		return nil, err
	}

	token, err := s.tokenProvider.GenerateToken(created.ID, created.Email)
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	return &AuthResponse{
		Token: token,
		User: UserResponse{
			ID:        created.ID,
			Name:      created.Name,
			Email:     created.Email,
			CreatedAt: created.CreatedAt,
		},
	}, nil
}

func (s *AuthService) Login(ctx context.Context, input LoginInput) (*AuthResponse, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))
	if email == "" {
		return nil, ErrUserEmailRequired
	}
	if input.Password == "" {
		return nil, ErrUserPasswordRequired
	}

	user, err := s.repo.GetByEmail(ctx, email)
	if errors.Is(err, ErrUserNotFound) {
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	token, err := s.tokenProvider.GenerateToken(user.ID, user.Email)
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	return &AuthResponse{
		Token: token,
		User: UserResponse{
			ID:        user.ID,
			Name:      user.Name,
			Email:     user.Email,
			CreatedAt: user.CreatedAt,
		},
	}, nil
}

func (s *AuthService) GetMe(ctx context.Context, userID uuid.UUID) (*UserResponse, error) {
	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &UserResponse{
		ID:        user.ID,
		Name:      user.Name,
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
	}, nil
}

func isValidEmail(email string) bool {
	addr, err := mail.ParseAddress(email)
	if err != nil {
		return false
	}
	if addr.Address != email {
		return false
	}
	parts := strings.Split(email, "@")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return false
	}
	domainParts := strings.Split(parts[1], ".")
	return len(domainParts) >= 2 && domainParts[len(domainParts)-1] != ""
}
