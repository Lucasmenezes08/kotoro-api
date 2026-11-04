package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestJWTTokenProvider(t *testing.T) {
	provider := NewJWTTokenProvider("test-secret-key-123", time.Hour)
	userID := uuid.New()
	email := "user@example.com"

	tokenString, err := provider.GenerateToken(userID, email)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	if tokenString == "" {
		t.Fatal("generated empty token string")
	}

	claims, err := provider.ValidateToken(tokenString)
	if err != nil {
		t.Fatalf("failed to validate token: %v", err)
	}
	if claims.UserID != userID || claims.Email != email {
		t.Fatalf("claims mismatch: got %v %v, want %v %v", claims.UserID, claims.Email, userID, email)
	}

	t.Run("rejects expired token", func(t *testing.T) {
		expiredProvider := NewJWTTokenProvider("test-secret-key-123", -time.Minute)
		expiredToken, err := expiredProvider.GenerateToken(userID, email)
		if err != nil {
			t.Fatalf("generate expired token: %v", err)
		}
		_, err = provider.ValidateToken(expiredToken)
		if err == nil {
			t.Fatal("expected expired token to fail validation")
		}
	})

	t.Run("rejects token with different secret", func(t *testing.T) {
		otherProvider := NewJWTTokenProvider("different-secret-key", time.Hour)
		otherToken, err := otherProvider.GenerateToken(userID, email)
		if err != nil {
			t.Fatalf("generate other token: %v", err)
		}
		_, err = provider.ValidateToken(otherToken)
		if err == nil {
			t.Fatal("expected validation failure with mismatched secret")
		}
	})
}
