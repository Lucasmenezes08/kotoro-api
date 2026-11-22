package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

type fakeAuthServiceForHandler struct {
	registerFn func(ctx context.Context, input RegisterInput) (*AuthResponse, error)
	loginFn    func(ctx context.Context, input LoginInput) (*AuthResponse, error)
	getMeFn    func(ctx context.Context, userID uuid.UUID) (*UserResponse, error)
}

func (f *fakeAuthServiceForHandler) Register(ctx context.Context, input RegisterInput) (*AuthResponse, error) {
	if f.registerFn != nil {
		return f.registerFn(ctx, input)
	}
	return &AuthResponse{Token: "test-token", User: UserResponse{ID: uuid.New(), Name: input.Name, Email: input.Email}}, nil
}

func (f *fakeAuthServiceForHandler) Login(ctx context.Context, input LoginInput) (*AuthResponse, error) {
	if f.loginFn != nil {
		return f.loginFn(ctx, input)
	}
	return &AuthResponse{Token: "test-token", User: UserResponse{ID: uuid.New(), Name: "Test", Email: input.Email}}, nil
}

func (f *fakeAuthServiceForHandler) GetMe(ctx context.Context, userID uuid.UUID) (*UserResponse, error) {
	if f.getMeFn != nil {
		return f.getMeFn(ctx, userID)
	}
	return &UserResponse{ID: userID, Name: "Test User", Email: "test@example.com", CreatedAt: time.Now()}, nil
}

func TestAuthHandlerRegister(t *testing.T) {
	handler := NewAuthHandler(&fakeAuthServiceForHandler{})

	t.Run("success returns 201", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader(`{"name":"Lucas","email":"lucas@example.com","password":"secretpassword"}`))
		rec := httptest.NewRecorder()

		handler.Register(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d", rec.Code)
		}
	})

	t.Run("invalid json returns 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader(`{invalid`))
		rec := httptest.NewRecorder()

		handler.Register(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("duplicate email returns 409", func(t *testing.T) {
		svc := &fakeAuthServiceForHandler{
			registerFn: func(ctx context.Context, input RegisterInput) (*AuthResponse, error) {
				return nil, ErrUserEmailAlreadyExists
			},
		}
		h := NewAuthHandler(svc)
		req := httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader(`{"name":"Lucas","email":"dup@example.com","password":"secretpassword"}`))
		rec := httptest.NewRecorder()

		h.Register(rec, req)

		if rec.Code != http.StatusConflict {
			t.Fatalf("expected 409, got %d", rec.Code)
		}
	})
}

func TestAuthHandlerLogin(t *testing.T) {
	handler := NewAuthHandler(&fakeAuthServiceForHandler{})

	t.Run("success returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"email":"lucas@example.com","password":"secretpassword"}`))
		rec := httptest.NewRecorder()

		handler.Login(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("invalid credentials returns 401", func(t *testing.T) {
		svc := &fakeAuthServiceForHandler{
			loginFn: func(ctx context.Context, input LoginInput) (*AuthResponse, error) {
				return nil, ErrInvalidCredentials
			},
		}
		h := NewAuthHandler(svc)
		req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"email":"lucas@example.com","password":"wrong"}`))
		rec := httptest.NewRecorder()

		h.Login(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})
}

func TestAuthHandlerGetMe(t *testing.T) {
	userID := uuid.New()
	handler := NewAuthHandler(&fakeAuthServiceForHandler{})

	t.Run("success with authenticated context", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
		req = req.WithContext(WithUserID(req.Context(), userID))
		rec := httptest.NewRecorder()

		handler.GetMe(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("unauthorized without user in context", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
		rec := httptest.NewRecorder()

		handler.GetMe(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})
}
