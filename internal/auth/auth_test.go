package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestMakeJWT(t *testing.T) {
	userID := uuid.New()
	secret := "test-secret-for-jwt"
	token, err := MakeJWT(userID, secret, 1*time.Hour)
	if err != nil {
		t.Fatalf("MakeJWT returned an error: %v", err)
	}
	if token == "" {
		t.Fatal("MakeJWT returned an empty token")
	}
	gotUserID, err := ValidateJWT(token, secret)
	if err != nil {
		t.Fatalf("ValidateJWT returned an error: %v", err)
	}
	if gotUserID != userID {
		t.Fatalf("ValidateJWT returned wrong userID: got %v, want %v", gotUserID, userID)
	}
}

func TestValidateJWT_InvalidToken(t *testing.T) {
	secret := "test-secret-for-jwt"
	invalidToken := "invalid.token.string"
	_, err := ValidateJWT(invalidToken, secret)
	if err == nil {
		t.Fatal("ValidateJWT did not return an error for an invalid token")
	}
}

func TestValidateJWT_EmptySecret(t *testing.T) {
	userID := uuid.New()
	secret := "test-secret-for-jwt"
	token, err := MakeJWT(userID, secret, 1*time.Hour)
	if err != nil {
		t.Fatalf("MakeJWT returned an error: %v", err)
	}
	_, err = ValidateJWT(token, "")
	if err == nil {
		t.Fatal("ValidateJWT did not return an error for an empty secret")
	}
}
