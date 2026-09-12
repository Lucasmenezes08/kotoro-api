package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/Lucasmenezes08/sprint-builder-api.git/internal/app"
	"github.com/Lucasmenezes08/sprint-builder-api.git/internal/database"
	"github.com/joho/godotenv"
)

func main() {
	if err := run(); err != nil {
		slog.Error("Application stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {

	_ = godotenv.Load()

	dbVariables, err := loadFromDatabase()
	if err != nil {
		return err
	}

	startupCtx, startupCancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)

	startupCancel()

	newDb, err := database.ConnectDatabase(startupCtx, dbVariables)
	if err != nil {
		return err
	}

	defer func() {
		if err := newDb.Close(); err != nil {
			slog.Error("failed to close database", "error", err)
		}
	}()

	slog.Info("database connection established")

	application := app.New()

	errCh := make(chan error, 1)
	go func() {
		slog.Info("application started", "addr", application.Addr())
		errCh <- application.ListenAndServe()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	defer signal.Stop(sigCh)

	select {
	case sig := <-sigCh:
		slog.Info("shutdown signal received", "signal", sig.String())
		signal.Stop(sigCh)

	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf(
				"HTTP server stopped unexpectedly: %w",
				err,
			)
		}
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)

	defer cancel()

	if err := application.Shutdown(ctx); err != nil {
		closeErr := application.Close()

		<-errCh

		return errors.Join(
			fmt.Errorf(
				"graceful shutdown deadline exceeded: %w",
				err,
			),
			closeErr,
		)

	}

	serverErr := <-errCh
	if !errors.Is(serverErr, http.ErrServerClosed) {
		return fmt.Errorf(
			"server failed during shutdown: %w",
			serverErr,
		)
	}

	return nil
}

func loadFromDatabase() (database.Config, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return database.Config{}, errors.New("DATABASE_URL is required")
	}
	maxOpenConns, err := envInt(
		"DB_MAX_OPEN_CONNS",
		10,
	)
	if err != nil {
		return database.Config{}, err
	}

	maxIdleConns, err := envInt(
		"DB_MAX_IDLE_CONNS",
		5,
	)
	if err != nil {
		return database.Config{}, err
	}

	return database.Config{
		URL:             dsn,
		MaxOpenConns:    maxOpenConns,
		MaxIdleConns:    maxIdleConns,
		ConnMaxLifetime: 30 * time.Minute,
		ConnMaxIdleTime: 5 * time.Minute,
	}, nil

}

func envInt(key string, fallback int) (int, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf(
			"%s must be an integer: %w",
			key,
			err,
		)
	}

	return parsed, nil
}
