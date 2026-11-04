package auth

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/Lucasmenezes08/kotoro-api.git/internal/utils"
)

type AuthHandler struct {
	service AuthServiceContract
}

func NewAuthHandler(service AuthServiceContract) *AuthHandler {
	return &AuthHandler{
		service: service,
	}
}

type registerRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&req); err != nil {
		utils.WriteJson(w, http.StatusBadRequest, authErrorResponse{Error: "Invalid request body"})
		return
	}

	resp, err := h.service.Register(r.Context(), RegisterInput{
		Name:     req.Name,
		Email:    req.Email,
		Password: req.Password,
	})

	switch {
	case errors.Is(err, ErrUserNameRequired),
		errors.Is(err, ErrUserEmailRequired),
		errors.Is(err, ErrUserEmailInvalid),
		errors.Is(err, ErrUserPasswordRequired),
		errors.Is(err, ErrUserPasswordTooShort):
		utils.WriteJson(w, http.StatusBadRequest, authErrorResponse{Error: err.Error()})
	case errors.Is(err, ErrUserEmailAlreadyExists):
		utils.WriteJson(w, http.StatusConflict, authErrorResponse{Error: err.Error()})
	case err != nil:
		slog.Error("failed to register user", "error", err)
		utils.WriteJson(w, http.StatusInternalServerError, authErrorResponse{Error: "Internal server error"})
	default:
		utils.WriteJson(w, http.StatusCreated, resp)
	}
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&req); err != nil {
		utils.WriteJson(w, http.StatusBadRequest, authErrorResponse{Error: "Invalid request body"})
		return
	}

	resp, err := h.service.Login(r.Context(), LoginInput{
		Email:    req.Email,
		Password: req.Password,
	})

	switch {
	case errors.Is(err, ErrUserEmailRequired),
		errors.Is(err, ErrUserPasswordRequired):
		utils.WriteJson(w, http.StatusBadRequest, authErrorResponse{Error: err.Error()})
	case errors.Is(err, ErrInvalidCredentials):
		utils.WriteJson(w, http.StatusUnauthorized, authErrorResponse{Error: err.Error()})
	case err != nil:
		slog.Error("failed to login user", "error", err)
		utils.WriteJson(w, http.StatusInternalServerError, authErrorResponse{Error: "Internal server error"})
	default:
		utils.WriteJson(w, http.StatusOK, resp)
	}
}

func (h *AuthHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	userID, err := GetUserIDFromContext(r.Context())
	if err != nil {
		utils.WriteJson(w, http.StatusUnauthorized, authErrorResponse{Error: "Unauthorized"})
		return
	}

	resp, err := h.service.GetMe(r.Context(), userID)
	switch {
	case errors.Is(err, ErrUserNotFound):
		utils.WriteJson(w, http.StatusNotFound, authErrorResponse{Error: "User not found"})
	case err != nil:
		slog.Error("failed to get current user", "error", err)
		utils.WriteJson(w, http.StatusInternalServerError, authErrorResponse{Error: "Internal server error"})
	default:
		utils.WriteJson(w, http.StatusOK, resp)
	}
}
