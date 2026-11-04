package app

import (
	"net/http"

	"github.com/Lucasmenezes08/kotoro-api.git/internal/auth"
	blocktasks "github.com/Lucasmenezes08/kotoro-api.git/internal/block-tasks"
	"github.com/Lucasmenezes08/kotoro-api.git/internal/sprint"
	sprintblocks "github.com/Lucasmenezes08/kotoro-api.git/internal/sprint-blocks"
	studycycles "github.com/Lucasmenezes08/kotoro-api.git/internal/study-cycles"
	"github.com/Lucasmenezes08/kotoro-api.git/internal/subject"
	"github.com/jmoiron/sqlx"
)

func RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
}

func RegisterAuthRoutes(mux *http.ServeMux, db *sqlx.DB, authMiddleware *auth.AuthMiddleware, tokenProvider auth.TokenProvider) {
	repository := auth.NewAuthRepository(db)
	service := auth.NewAuthService(repository, tokenProvider)
	handler := auth.NewAuthHandler(service)

	mux.HandleFunc("POST /auth/register", handler.Register)
	mux.HandleFunc("POST /auth/login", handler.Login)
	mux.HandleFunc("GET /auth/me", authMiddleware.RequireAuth(handler.GetMe))
}

func RegisterBlockTaskRoutes(mux *http.ServeMux, db *sqlx.DB, authMiddleware *auth.AuthMiddleware) {
	repository := blocktasks.NewBlockTaskRepository(db)
	sprintRepository := sprint.NewSprintRepository(db)
	blockRepository := sprintblocks.NewSprintBlockRepository(db)
	service := blocktasks.NewBlockTaskService(repository, sprintRepository, blockRepository)
	handler := blocktasks.NewBlockTaskHandler(service)

	mux.HandleFunc("POST /sprints/{sprintID}/blocks/{blockID}/tasks", authMiddleware.RequireAuth(handler.Create))
	mux.HandleFunc("GET /sprints/{sprintID}/blocks/{blockID}/tasks", authMiddleware.RequireAuth(handler.GetAll))
	mux.HandleFunc("PUT /sprints/{sprintID}/blocks/{blockID}/tasks/reorder", authMiddleware.RequireAuth(handler.Reorder))
	mux.HandleFunc("GET /sprints/{sprintID}/blocks/{blockID}/tasks/{taskID}", authMiddleware.RequireAuth(handler.GetByID))
	mux.HandleFunc("PATCH /sprints/{sprintID}/blocks/{blockID}/tasks/{taskID}", authMiddleware.RequireAuth(handler.Update))
	mux.HandleFunc("DELETE /sprints/{sprintID}/blocks/{blockID}/tasks/{taskID}", authMiddleware.RequireAuth(handler.Delete))
}

func RegisterSprintBlockRoutes(mux *http.ServeMux, db *sqlx.DB, authMiddleware *auth.AuthMiddleware) {
	repository := sprintblocks.NewSprintBlockRepository(db)
	sprintRepository := sprint.NewSprintRepository(db)
	subjectRepository := subject.NewSubjectRepository(db)
	service := sprintblocks.NewSprintBlockService(repository, sprintRepository, subjectRepository)
	handler := sprintblocks.NewSprintBlockHandler(service)

	mux.HandleFunc("POST /sprints/{sprintID}/blocks", authMiddleware.RequireAuth(handler.Create))
	mux.HandleFunc("GET /sprints/{sprintID}/blocks", authMiddleware.RequireAuth(handler.GetAll))
	mux.HandleFunc("PUT /sprints/{sprintID}/blocks/reorder", authMiddleware.RequireAuth(handler.Reorder))
	mux.HandleFunc("GET /sprints/{sprintID}/blocks/{blockID}", authMiddleware.RequireAuth(handler.GetByID))
	mux.HandleFunc("PATCH /sprints/{sprintID}/blocks/{blockID}", authMiddleware.RequireAuth(handler.Update))
	mux.HandleFunc("DELETE /sprints/{sprintID}/blocks/{blockID}", authMiddleware.RequireAuth(handler.Delete))
}

func RegisterSubjectsRoutes(mux *http.ServeMux, db *sqlx.DB, authMiddleware *auth.AuthMiddleware) {
	repository := subject.NewSubjectRepository(db)
	service := subject.NewSubjectService(repository)
	controller := subject.NewSubjectController(service)

	mux.HandleFunc("GET /subjects", authMiddleware.RequireAuth(controller.GetAll))
	mux.HandleFunc("POST /subjects", authMiddleware.RequireAuth(controller.Create))
	mux.HandleFunc("PATCH /subjects/{id}", authMiddleware.RequireAuth(controller.Update))
	mux.HandleFunc("DELETE /subjects/{id}", authMiddleware.RequireAuth(controller.DeleteById))
	mux.HandleFunc("POST /subjects/import", authMiddleware.RequireAuth(controller.CreateBatch))
}

func RegisterSprintRoutes(mux *http.ServeMux, db *sqlx.DB, authMiddleware *auth.AuthMiddleware) {
	repository := sprint.NewSprintRepository(db)
	service := sprint.NewSprintService(repository)
	controller := sprint.NewSprintHandler(service)

	mux.HandleFunc("GET /sprints", authMiddleware.RequireAuth(controller.GetAll))
	mux.HandleFunc("GET /sprints/{id}", authMiddleware.RequireAuth(controller.GetById))
	mux.HandleFunc("POST /sprints", authMiddleware.RequireAuth(controller.Create))
	mux.HandleFunc("PATCH /sprints/{id}", authMiddleware.RequireAuth(controller.Update))
	mux.HandleFunc("DELETE /sprints/{id}", authMiddleware.RequireAuth(controller.DeleteById))
}

func RegisterStudyCycleRoutes(mux *http.ServeMux, db *sqlx.DB, authMiddleware *auth.AuthMiddleware) {
	repository := studycycles.NewStudyCycleRepository(db)
	service := studycycles.NewStudyCycleService(repository)
	handler := studycycles.NewStudyCycleHandler(service)

	mux.HandleFunc("POST /sprints/{sprintID}/study-cycles/play", authMiddleware.RequireAuth(handler.Play))
	mux.HandleFunc("POST /study-cycles/{cycleID}/pause", authMiddleware.RequireAuth(handler.Pause))
	mux.HandleFunc("POST /study-cycles/{cycleID}/resume", authMiddleware.RequireAuth(handler.Resume))
	mux.HandleFunc("POST /study-cycles/{cycleID}/stop", authMiddleware.RequireAuth(handler.Stop))
}
