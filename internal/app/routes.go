package app

import (
	"net/http"

	"github.com/Lucasmenezes08/kotoro-api.git/internal/subject"
	"github.com/jmoiron/sqlx"
)


func RegisterRoutes(mux *http.ServeMux){
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
}

func RegisterSubjectsRoutes(mux * http.ServeMux, db *sqlx.DB){
	repository := subject.NewSubjectRepository(db)
	service := subject.NewSubjectService(repository)
	controller := subject.NewSubjectController(service)

	mux.HandleFunc("GET /subjects", controller.GetAll)
	mux.HandleFunc("POST /subjects", controller.Create)
}