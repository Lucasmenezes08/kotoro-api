package app

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/Lucasmenezes08/kotoro-api.git/internal/auth"
	"github.com/jmoiron/sqlx"
)

type Application struct {
	server *http.Server
}

func New(db *sqlx.DB) *Application {
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "kotoro-default-jwt-secret-key-change-me-in-production"
	}
	tokenProvider := auth.NewJWTTokenProvider(jwtSecret, 24*time.Hour)
	authMiddleware := auth.NewAuthMiddleware(tokenProvider)

	mux := http.NewServeMux()
	RegisterRoutes(mux)
	RegisterAuthRoutes(mux, db, authMiddleware, tokenProvider)
	RegisterSubjectsRoutes(mux, db, authMiddleware)
	RegisterSprintRoutes(mux, db, authMiddleware)
	RegisterSprintBlockRoutes(mux, db, authMiddleware)
	RegisterBlockTaskRoutes(mux, db, authMiddleware)
	RegisterStudyCycleRoutes(mux, db, authMiddleware)

	handler := CorsMiddleware(mux)

	return &Application{
		server: &http.Server{
			Addr:              ":8080",
			Handler:           handler,
			ReadHeaderTimeout: 5 * time.Second,
			WriteTimeout:      10 * time.Second,
			IdleTimeout:       60 * time.Second,
		},
	}
}

func (a *Application) ListenAndServe() error {
	return a.server.ListenAndServe()
}

func (a *Application) Addr() string {
	return a.server.Addr
}

func (a *Application) Shutdown(ctx context.Context) error {
	return a.server.Shutdown(ctx)
}

func (a *Application) Close() error {
	return a.server.Close()
}
