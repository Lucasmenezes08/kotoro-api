package app

import (
	"net/http"

	"github.com/Lucasmenezes08/kotoro-api.git/internal/sprint"
	sprintblocks "github.com/Lucasmenezes08/kotoro-api.git/internal/sprint-blocks"
	"github.com/Lucasmenezes08/kotoro-api.git/internal/subject"
	"github.com/jmoiron/sqlx"
)

func RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
}

func RegisterSprintBlockRoutes(mux *http.ServeMux, db *sqlx.DB) {
	repository := sprintblocks.NewSprintBlockRepository(db)
	sprintRepository := sprint.NewSprintRepository(db)
	subjectRepository := subject.NewSubjectRepository(db)
	service := sprintblocks.NewSprintBlockService(repository, sprintRepository, subjectRepository)
	handler := sprintblocks.NewSprintBlockHandler(service)

	mux.HandleFunc("POST /sprints/{sprintID}/blocks", handler.Create)
	mux.HandleFunc("GET /sprints/{sprintID}/blocks", handler.GetAll)
	mux.HandleFunc("PUT /sprints/{sprintID}/blocks/reorder", handler.Reorder)
	mux.HandleFunc("GET /sprints/{sprintID}/blocks/{blockID}", handler.GetByID)
	mux.HandleFunc("PATCH /sprints/{sprintID}/blocks/{blockID}", handler.Update)
	mux.HandleFunc("DELETE /sprints/{sprintID}/blocks/{blockID}", handler.Delete)
}

func RegisterSubjectsRoutes(mux *http.ServeMux, db *sqlx.DB) {
	repository := subject.NewSubjectRepository(db)
	service := subject.NewSubjectService(repository)
	controller := subject.NewSubjectController(service)

	mux.HandleFunc("GET /subjects", controller.GetAll)
	mux.HandleFunc("POST /subjects", controller.Create)
	mux.HandleFunc("PATCH /subjects/{id}", controller.Update)
	mux.HandleFunc("DELETE /subjects/{id}", controller.DeleteById)
	mux.HandleFunc("POST /subjects/import", controller.CreateBatch)
}

func RegisterSprintRoutes(mux *http.ServeMux, db *sqlx.DB) {
	repository := sprint.NewSprintRepository(db)
	service := sprint.NewSprintService(repository)
	controller := sprint.NewSprintHandler(service)

	mux.HandleFunc("GET /sprints", controller.GetAll)
	mux.HandleFunc("GET /sprints/{id}", controller.GetById)
	mux.HandleFunc("POST /sprints", controller.Create)
	mux.HandleFunc("PATCH /sprints/{id}", controller.Update)
	mux.HandleFunc("DELETE /sprints/{id}", controller.DeleteById)
}
