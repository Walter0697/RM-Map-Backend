package utils

import (
	"errors"
	"fmt"
	"log"
	"mapmarker/backend/config"
	"strings"
	"time"

	"github.com/golang-jwt/jwt"
)

type JWTInfo struct {
	Username string
	Secret   string
}

func GenerateToken(username string, secret string) (string, error) {
	issuedAt := time.Now().UTC()
	tokenTTLSeconds := config.ResolveAuthTokenLifetimeSeconds()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"username": strings.TrimSpace(username),
		"secret":   strings.TrimSpace(secret),
		"iat":      issuedAt.Unix(),
		"exp":      issuedAt.Add(time.Duration(tokenTTLSeconds) * time.Second).Unix(),
	})

	tokenString, err := token.SignedString([]byte(config.Data.App.JWT))
	if err != nil {
		log.Fatal("Error in generating key for " + username)
		return "", err
	}
	return tokenString, nil
}

func ParseToken(tokenStr string) (*JWTInfo, error) {
	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("invalid signing method")
		}
		if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("unsupported signing method: %s", token.Method.Alg())
		}
		return []byte(config.Data.App.JWT), nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	usernameRaw, hasUsername := claims["username"]
	secretRaw, hasSecret := claims["secret"]
	if !hasUsername || !hasSecret {
		return nil, fmt.Errorf("invalid token claims")
	}

	username, okUsername := usernameRaw.(string)
	secret, okSecret := secretRaw.(string)
	if !okUsername || strings.TrimSpace(username) == "" {
		return nil, fmt.Errorf("invalid token username")
	}
	if !okSecret || strings.TrimSpace(secret) == "" {
		return nil, fmt.Errorf("invalid token secret")
	}

	return &JWTInfo{Username: strings.TrimSpace(username), Secret: strings.TrimSpace(secret)}, nil
}
