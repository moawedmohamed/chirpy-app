package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/alexedwards/argon2id"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func HashPassword(password string) (string, error) {
	hashed, err := argon2id.CreateHash(password, argon2id.DefaultParams)
	if err != nil {
		return "", err
	}
	return hashed, nil
}

func CheckPasswordHash(password, hash string) (bool, error) {
	return argon2id.ComparePasswordAndHash(password, hash)
}

func MakeJWT(userID uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error) {
	if tokenSecret == "" {
		return "", errors.New("token secret is empty")
	}
	claims := jwt.RegisteredClaims{
		Issuer:    "chirpy-access",
		Subject:   userID.String(),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiresIn)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signToken, err := token.SignedString([]byte(tokenSecret))
	if err != nil {
		return "", err
	}
	return signToken, nil
}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error) {
	if tokenSecret == "" {
		return uuid.Nil, errors.New("token secret is empty")
	}

	if tokenSecret == "" {
		return uuid.Nil, errors.New("TOKEN_SECRET is not set")
	}
	claims := &jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(tokenString,
		claims,
		func(token *jwt.Token) (any, error) {
			return []byte(tokenSecret), nil
		},
		jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithIssuer("chirpy-access"))
	if err != nil {
		return uuid.UUID{}, err
	}
	if !token.Valid {
		return uuid.Nil, errors.New("invalid token")
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		log.Println("Error parsing user ID from token:", err)
		return uuid.Nil, errors.New("invalid user ID in token")
	}

	return userID, nil
}

func GetBearerToken(headers http.Header) (string, error) {
	bearerToken := headers.Get("Authorization")
	if bearerToken == "" {
		return "", errors.New("authorization header is missing")
	}

	prefix := "Bearer "
	if len(bearerToken) <= len(prefix) || bearerToken[:len(prefix)] != prefix {
		return "", errors.New("invalid authorization header format")
	}
	return bearerToken[len(prefix):], nil
}

func MakeRefreshToken() string {
	randamBytes := make([]byte, 32)
	_, err := rand.Read(randamBytes)
	if err != nil {
		return ""
	}
	return hex.EncodeToString(randamBytes)
}

func GetAPIKey(headers http.Header) (string, error) {
	apiKey := headers.Get("Authorization")
	if apiKey == "" {
		return "", errors.New("api key header is missing")
	}
	prefix := "ApiKey "
	if len(apiKey) <= len(prefix) || apiKey[:len(prefix)] != prefix {
		return "", errors.New("invalid api key header format")
	}
	apiKey = apiKey[len(prefix):]
	return apiKey, nil
}
