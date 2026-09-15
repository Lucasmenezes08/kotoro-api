package app

import (
	"context"
	"net/http"
	"time"

	"github.com/jmoiron/sqlx"
)


type Application struct {
	server *http.Server
}


func New(db *sqlx.DB) *Application {
	mux := http.NewServeMux()
	RegisterRoutes(mux)
	RegisterSubjectsRoutes(mux, db)

	return &Application{
		server : &http.Server{
			Addr:              ":8080",
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout: 60 * time.Second,
		},
	}	
}

func (a *Application)ListenAndServe()error{
	return a.server.ListenAndServe()
}

func (a *Application)Addr()string{
	return a.server.Addr
}

func (a *Application)Shutdown(ctx context.Context)error{
	return a.server.Shutdown(ctx)
}

func (a *Application)Close()error{
	return a.server.Close()
}