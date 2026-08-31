package handler

import (
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func init() {
	if os.Getenv("JWT_SECRET") == "" {
		jwtSecret = []byte("test-jwt-secret-key-1234567890123456")
	}
}

func TestGenerateAndValidateToken(t *testing.T) {
	userID := "user-12345"
	username := "testuser"

	tokenStr, err := GenerateToken(userID, username)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}
	if tokenStr == "" {
		t.Fatal("GenerateToken returned empty token")
	}

	claims, err := ValidateToken(tokenStr)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}

	if claims.UserID != userID {
		t.Errorf("Expected UserID %q, got %q", userID, claims.UserID)
	}
	if claims.Username != username {
		t.Errorf("Expected Username %q, got %q", username, claims.Username)
	}
	if claims.IsAdmin {
		t.Error("Expected IsAdmin to be false for regular user token")
	}
}

func TestGenerateAndValidateAdminToken(t *testing.T) {
	adminID := "admin-999"
	adminUser := "superadmin"

	tokenStr, err := GenerateAdminToken(adminID, adminUser)
	if err != nil {
		t.Fatalf("GenerateAdminToken failed: %v", err)
	}

	claims, err := ValidateToken(tokenStr)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}

	if claims.UserID != adminID {
		t.Errorf("Expected UserID %q, got %q", adminID, claims.UserID)
	}
	if claims.Username != adminUser {
		t.Errorf("Expected Username %q, got %q", adminUser, claims.Username)
	}
	if !claims.IsAdmin {
		t.Error("Expected IsAdmin to be true for admin token")
	}
}

func TestValidateToken_Invalid(t *testing.T) {
	invalidTokens := []string{
		"",
		"invalid.token.string",
		"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.e30.invalid_signature",
	}

	for _, token := range invalidTokens {
		_, err := ValidateToken(token)
		if err == nil {
			t.Errorf("Expected error for invalid token %q, got nil", token)
		}
	}
}

func TestValidateToken_Expired(t *testing.T) {
	// Create an expired token manually
	expiredClaims := &Claims{
		UserID:   "expired-user",
		Username: "expired",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, expiredClaims)
	tokenStr, err := token.SignedString(jwtSecret)
	if err != nil {
		t.Fatalf("Failed to sign token: %v", err)
	}

	_, err = ValidateToken(tokenStr)
	if err == nil {
		t.Error("Expected error for expired token, got nil")
	}
}

func TestIsValidHexColor(t *testing.T) {
	tests := []struct {
		color    string
		expected bool
	}{
		{"#fff", true},
		{"#FFF", true},
		{"#000000", true},
		{"#4e342e", true},
		{"#4E342E", true},
		{"#123456", true},
		{"", false},
		{"#", false},
		{"#ff", false},
		{"#ffff", false},
		{"#ffffff0", false},
		{"#gggggg", false},
		{"red", false},
		{"#12345z", false},
	}

	for _, tt := range tests {
		got := isValidHexColor(tt.color)
		if got != tt.expected {
			t.Errorf("isValidHexColor(%q) = %v, expected %v", tt.color, got, tt.expected)
		}
	}
}
