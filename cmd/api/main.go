package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	server := &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout: 60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func(){
		slog.Info("Server started", "addr", server.Addr)
		errCh <- server.ListenAndServe()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	defer signal.Stop(sigCh)


	select {
	case sig := <-sigCh:
		slog.Info("End signal received", "signal", sig.String())
	
	case err := <- errCh:
		if !errors.Is(err, http.ErrServerClosed){
			slog.Error("Server forced stop with error", "error", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25 * time.Second)

	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		slog.Error("shutown time exceeted", "error", err)
		_ = server.Close()
	}
	slog.Info("Shutdown done")
}
