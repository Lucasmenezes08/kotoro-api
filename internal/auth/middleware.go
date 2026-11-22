package auth

import (
	"net/http"
	"strings"

	"github.com/Lucasmenezes08/kotoro-api.git/internal/utils"
)

type AuthMiddleware struct {
	tokenProvider TokenProvider
}

func NewAuthMiddleware(tokenProvider TokenProvider) *AuthMiddleware {
	return &AuthMiddleware{
		tokenProvider: tokenProvider,
	}
}

type authErrorResponse struct {
	Error string `json:"error"`
}

func (m *AuthMiddleware) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			utils.WriteJson(w, http.StatusUnauthorized, authErrorResponse{Error: "Unauthorized"})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			utils.WriteJson(w, http.StatusUnauthorized, authErrorResponse{Error: "Unauthorized"})
			return
		}

		tokenString := strings.TrimSpace(parts[1])
		if tokenString == "" {
			utils.WriteJson(w, http.StatusUnauthorized, authErrorResponse{Error: "Unauthorized"})
			return
		}

		claims, err := m.tokenProvider.ValidateToken(tokenString)
		if err != nil {
			utils.WriteJson(w, http.StatusUnauthorized, authErrorResponse{Error: "Unauthorized"})
			return
		}

		ctx := WithUserID(r.Context(), claims.UserID)
		next(w, r.WithContext(ctx))
	}
}
