package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type fakeTokenProvider struct {
	generateFn func(userID uuid.UUID, email string) (string, error)
	validateFn func(tokenString string) (*TokenClaims, error)
}

func (f *fakeTokenProvider) GenerateToken(userID uuid.UUID, email string) (string, error) {
	if f.generateFn != nil {
		return f.generateFn(userID, email)
	}
	return "fake-token-" + userID.String(), nil
}

func (f *fakeTokenProvider) ValidateToken(tokenString string) (*TokenClaims, error) {
	if f.validateFn != nil {
		return f.validateFn(tokenString)
	}
	return &TokenClaims{UserID: uuid.New(), Email: "test@example.com"}, nil
}

type fakeAuthRepository struct {
	createFn        func(ctx context.Context, user User) (*User, error)
	getByIDFn       func(ctx context.Context, id uuid.UUID) (*User, error)
	getByEmailFn    func(ctx context.Context, email string) (*User, error)
	existsByEmailFn func(ctx context.Context, email string) (bool, error)
}

func (f *fakeAuthRepository) Create(ctx context.Context, user User) (*User, error) {
	if f.createFn != nil {
		return f.createFn(ctx, user)
	}
	user.ID = uuid.New()
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()
	return &user, nil
}

func (f *fakeAuthRepository) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	if f.getByIDFn != nil {
		return f.getByIDFn(ctx, id)
	}
	return &User{ID: id, Name: "Test User", Email: "test@example.com"}, nil
}

func (f *fakeAuthRepository) GetByEmail(ctx context.Context, email string) (*User, error) {
	if f.getByEmailFn != nil {
		return f.getByEmailFn(ctx, email)
	}
	return nil, ErrUserNotFound
}

func (f *fakeAuthRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	if f.existsByEmailFn != nil {
		return f.existsByEmailFn(ctx, email)
	}
	return false, nil
}

func TestAuthServiceRegister(t *testing.T) {
	t.Run("successfully registers user", func(t *testing.T) {
		repo := &fakeAuthRepository{
			existsByEmailFn: func(ctx context.Context, email string) (bool, error) {
				return false, nil
			},
			createFn: func(ctx context.Context, user User) (*User, error) {
				if user.Name != "Lucas" || user.Email != "lucas@example.com" {
					t.Fatalf("unexpected user create input: %#v", user)
				}
				if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte("securepass")); err != nil {
					t.Fatalf("invalid password hash: %v", err)
				}
				user.ID = uuid.New()
				user.CreatedAt = time.Now()
				return &user, nil
			},
		}
		tokenProv := &fakeTokenProvider{
			generateFn: func(userID uuid.UUID, email string) (string, error) {
				return "jwt-token-123", nil
			},
		}
		service := NewAuthService(repo, tokenProv)

		resp, err := service.Register(context.Background(), RegisterInput{
			Name:     "  Lucas  ",
			Email:    "  LUCAS@example.com  ",
			Password: "securepass",
		})
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}
		if resp.Token != "jwt-token-123" || resp.User.Name != "Lucas" || resp.User.Email != "lucas@example.com" {
			t.Fatalf("unexpected response: %#v", resp)
		}
	})

	t.Run("validations", func(t *testing.T) {
		service := NewAuthService(&fakeAuthRepository{}, &fakeTokenProvider{})
		tests := []struct {
			name  string
			input RegisterInput
			want  error
		}{
			{"empty name", RegisterInput{Name: "   ", Email: "a@b.com", Password: "secret"}, ErrUserNameRequired},
			{"empty email", RegisterInput{Name: "Lucas", Email: "   ", Password: "secret"}, ErrUserEmailRequired},
			{"invalid email", RegisterInput{Name: "Lucas", Email: "invalid-email", Password: "secret"}, ErrUserEmailInvalid},
			{"empty password", RegisterInput{Name: "Lucas", Email: "a@b.com", Password: ""}, ErrUserPasswordRequired},
			{"short password", RegisterInput{Name: "Lucas", Email: "a@b.com", Password: "123"}, ErrUserPasswordTooShort},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				_, err := service.Register(context.Background(), tc.input)
				if !errors.Is(err, tc.want) {
					t.Fatalf("expected %v, got %v", tc.want, err)
				}
			})
		}
	})

	t.Run("email already exists", func(t *testing.T) {
		repo := &fakeAuthRepository{
			existsByEmailFn: func(ctx context.Context, email string) (bool, error) {
				return true, nil
			},
		}
		service := NewAuthService(repo, &fakeTokenProvider{})
		_, err := service.Register(context.Background(), RegisterInput{
			Name:     "Lucas",
			Email:    "lucas@example.com",
			Password: "securepassword",
		})
		if !errors.Is(err, ErrUserEmailAlreadyExists) {
			t.Fatalf("expected ErrUserEmailAlreadyExists, got %v", err)
		}
	})
}

func TestAuthServiceLogin(t *testing.T) {
	passHash, _ := bcrypt.GenerateFromPassword([]byte("my-secret"), bcrypt.DefaultCost)
	userID := uuid.New()
	existingUser := &User{
		ID:           userID,
		Name:         "Lucas",
		Email:        "lucas@example.com",
		PasswordHash: string(passHash),
		CreatedAt:    time.Now(),
	}

	t.Run("successful login", func(t *testing.T) {
		repo := &fakeAuthRepository{
			getByEmailFn: func(ctx context.Context, email string) (*User, error) {
				if email == "lucas@example.com" {
					return existingUser, nil
				}
				return nil, ErrUserNotFound
			},
		}
		service := NewAuthService(repo, &fakeTokenProvider{})
		resp, err := service.Login(context.Background(), LoginInput{
			Email:    "LUCAS@EXAMPLE.COM",
			Password: "my-secret",
		})
		if err != nil {
			t.Fatalf("login failed: %v", err)
		}
		if resp.User.ID != userID || resp.Token == "" {
			t.Fatalf("unexpected login response: %#v", resp)
		}
	})

	t.Run("invalid password", func(t *testing.T) {
		repo := &fakeAuthRepository{
			getByEmailFn: func(ctx context.Context, email string) (*User, error) {
				return existingUser, nil
			},
		}
		service := NewAuthService(repo, &fakeTokenProvider{})
		_, err := service.Login(context.Background(), LoginInput{
			Email:    "lucas@example.com",
			Password: "wrong-password",
		})
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("expected ErrInvalidCredentials, got %v", err)
		}
	})

	t.Run("user not found", func(t *testing.T) {
		repo := &fakeAuthRepository{
			getByEmailFn: func(ctx context.Context, email string) (*User, error) {
				return nil, ErrUserNotFound
			},
		}
		service := NewAuthService(repo, &fakeTokenProvider{})
		_, err := service.Login(context.Background(), LoginInput{
			Email:    "nonexistent@example.com",
			Password: "any-password",
		})
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("expected ErrInvalidCredentials, got %v", err)
		}
	})
}
