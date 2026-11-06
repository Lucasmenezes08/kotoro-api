package auth

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type TokenClaims struct {
	UserID uuid.UUID `json:"user_id"`
	Email  string    `json:"email"`
	jwt.RegisteredClaims
}

type TokenProvider interface {
	GenerateToken(userID uuid.UUID, email string) (string, error)
	ValidateToken(tokenString string) (*TokenClaims, error)
}

type JWTTokenProvider struct {
	secret   []byte
	duration time.Duration
}

const defaultJWTSecret = "kotoro-api-default-jwt-secret-key-change-in-production"

func NewJWTTokenProvider(secret string, duration time.Duration) *JWTTokenProvider {
	if secret == "" {
		secret = os.Getenv("JWT_SECRET")
	}
	if secret == "" {
		secret = defaultJWTSecret
	}
	if duration == 0 {
		duration = 24 * time.Hour
	}
	return &JWTTokenProvider{
		secret:   []byte(secret),
		duration: duration,
	}
}

func (p *JWTTokenProvider) GenerateToken(userID uuid.UUID, email string) (string, error) {
	now := time.Now().UTC()
	claims := TokenClaims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(p.duration)),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "kotoro-api",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(p.secret)
	if err != nil {
		return "", fmt.Errorf("sign jwt: %w", err)
	}
	return tokenString, nil
}

func (p *JWTTokenProvider) ValidateToken(tokenString string) (*TokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return p.secret, nil
	})

	if err != nil {
		return nil, errors.Join(ErrUnauthorized, err)
	}

	claims, ok := token.Claims.(*TokenClaims)
	if !ok || !token.Valid {
		return nil, ErrUnauthorized
	}

	if claims.UserID == uuid.Nil {
		return nil, ErrUnauthorized
	}

	return claims, nil
}
