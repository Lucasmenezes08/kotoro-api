package app

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Lucasmenezes08/kotoro-api.git/internal/auth"
)

func TestRoutes_AuthProtection(t *testing.T) {
	tokenProvider := auth.NewJWTTokenProvider("test-secret", time.Hour)
	authMiddleware := auth.NewAuthMiddleware(tokenProvider)

	mux := http.NewServeMux()
	RegisterRoutes(mux)
	RegisterAuthRoutes(mux, nil, authMiddleware, tokenProvider)
	RegisterSubjectsRoutes(mux, nil, authMiddleware)
	RegisterSprintRoutes(mux, nil, authMiddleware)
	RegisterSprintBlockRoutes(mux, nil, authMiddleware)
	RegisterBlockTaskRoutes(mux, nil, authMiddleware)
	RegisterStudyCycleRoutes(mux, nil, authMiddleware)

	t.Run("public endpoints do not require authorization", func(t *testing.T) {
		publicPaths := []struct {
			method string
			path   string
		}{
			{http.MethodGet, "/healthz"},
		}

		for _, p := range publicPaths {
			req := httptest.NewRequest(p.method, p.path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code == http.StatusUnauthorized {
				t.Fatalf("endpoint %s %s should not require authorization, got 401", p.method, p.path)
			}
		}
	})

	t.Run("protected endpoints require authorization and return 401 when token is missing", func(t *testing.T) {
		protectedPaths := []struct {
			method string
			path   string
		}{
			{http.MethodGet, "/auth/me"},
			{http.MethodGet, "/subjects"},
			{http.MethodPost, "/subjects"},
			{http.MethodGet, "/sprints"},
			{http.MethodPost, "/sprints"},
			{http.MethodGet, "/sprints/c379a04a-8745-42d4-a7fc-2e6b7fc5ef02/blocks"},
			{http.MethodPost, "/sprints/c379a04a-8745-42d4-a7fc-2e6b7fc5ef02/blocks"},
			{http.MethodGet, "/sprints/c379a04a-8745-42d4-a7fc-2e6b7fc5ef02/blocks/c379a04a-8745-42d4-a7fc-2e6b7fc5ef02/tasks"},
			{http.MethodPost, "/sprints/c379a04a-8745-42d4-a7fc-2e6b7fc5ef02/study-cycles/play"},
			{http.MethodPost, "/study-cycles/c379a04a-8745-42d4-a7fc-2e6b7fc5ef02/pause"},
			{http.MethodPost, "/study-cycles/c379a04a-8745-42d4-a7fc-2e6b7fc5ef02/resume"},
			{http.MethodPost, "/study-cycles/c379a04a-8745-42d4-a7fc-2e6b7fc5ef02/stop"},
		}

		for _, p := range protectedPaths {
			req := httptest.NewRequest(p.method, p.path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("expected 401 Unauthorized for %s %s without token, got %d", p.method, p.path, rec.Code)
			}
		}
	})
}
