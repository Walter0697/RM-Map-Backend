package utils

import (
	"log"
	"mapmarker/backend/config"

	"github.com/golang-jwt/jwt"
)

type JWTInfo struct {
	Username string
	Secret   string
}

func GenerateToken(username string, secret string) (string, error) {
	token := jwt.New(jwt.SigningMethodHS256)

	claims := token.Claims.(jwt.MapClaims)

	claims["username"] = username
	claims["secret"] = secret
	tokenString, err := token.SignedString([]byte(config.Data.App.JWT))
	if err != nil {
		log.Fatal("Error in generating key for " + username)
		return "", err
	}
	return tokenString, nil
}

func ParseToken(tokenStr string) (*JWTInfo, error) {
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		return []byte(config.Data.App.JWT), nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		username := claims["username"].(string)
		secret := claims["secret"].(string)
		result := JWTInfo{
			Username: username,
			Secret:   secret,
		}
		return &result, nil
	} else {
		return nil, err
	}
}
