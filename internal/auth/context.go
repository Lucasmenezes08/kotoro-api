package auth

import (
	"context"

	"github.com/google/uuid"
)

type contextKey string

const userIDContextKey contextKey = "kotoro_user_id"

func WithUserID(ctx context.Context, userID uuid.UUID) context.Context {
	return context.WithValue(ctx, userIDContextKey, userID)
}

func GetUserIDFromContext(ctx context.Context) (uuid.UUID, error) {
	val := ctx.Value(userIDContextKey)
	if val == nil {
		return uuid.Nil, ErrUnauthorized
	}
	userID, ok := val.(uuid.UUID)
	if !ok || userID == uuid.Nil {
		return uuid.Nil, ErrUnauthorized
	}
	return userID, nil
}
