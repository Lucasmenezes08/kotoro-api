package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestAuthMiddleware(t *testing.T) {
	provider := NewJWTTokenProvider("middleware-secret", time.Hour)
	middleware := NewAuthMiddleware(provider)

	userID := uuid.New()
	validToken, err := provider.GenerateToken(userID, "user@example.com")
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	handlerCalled := false
	testHandler := middleware.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		gotUserID, err := GetUserIDFromContext(r.Context())
		if err != nil || gotUserID != userID {
			t.Fatalf("context userID mismatch: got %v %v, want %v", gotUserID, err, userID)
		}
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	t.Run("allows request with valid bearer token", func(t *testing.T) {
		handlerCalled = false
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+validToken)
		rec := httptest.NewRecorder()

		testHandler(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		if !handlerCalled {
			t.Fatal("expected handler to be called")
		}
	})

	t.Run("rejects request without authorization header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		rec := httptest.NewRecorder()

		testHandler(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("rejects request with invalid token format", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Basic 123456")
		rec := httptest.NewRecorder()

		testHandler(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("rejects request with tampered token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer invalid.jwt.token")
		rec := httptest.NewRecorder()

		testHandler(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})
}
