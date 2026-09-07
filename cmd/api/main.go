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

	"github.com/Lucasmenezes08/sprint-builder-api.git/internal/app"
)

func main() {
	
	application := app.New()

	errCh := make(chan error, 1)
	go func(){
		slog.Info("application started", application.Addr(), application.Addr)
		errCh <- application.ListenAndServe()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	defer signal.Stop(sigCh)


	select {
	case sig := <-sigCh:
		slog.Info("End signal received", "signal", sig.String())
	
	case err := <- errCh:
		if !errors.Is(err, http.ErrServerClosed){
			slog.Error("application stopped unexpectedly", "error", err)
		}
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25 * time.Second)

	defer cancel()

	if err := application.Shutdown(ctx); err != nil {
		slog.Error("shutown time exceeted", "error", err)
		_ = application.Close()
	}
	slog.Info("Shutdown done")
}
