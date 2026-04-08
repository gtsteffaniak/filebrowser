package auth_test

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/gtsteffaniak/filebrowser/backend/auth"
)

func TestVerifyExternalJWT_ValidToken(t *testing.T) {
	secret := "test-secret-key"
	algorithm := "HS256"
	username := "testuser"

	// Create a valid JWT token
	claims := jwt.MapClaims{
		"sub":   username,
		"email": "test@example.com",
		"exp":   time.Now().Add(time.Hour).Unix(),
		"iat":   time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("Failed to create test token: %v", err)
	}

	// Verify the token
	extractedUsername, extractedClaims, err := auth.VerifyExternalJWT(tokenString, secret, algorithm, "sub")
	if err != nil {
		t.Fatalf("VerifyExternalJWT() failed: %v", err)
	}

	if extractedUsername != username {
		t.Errorf("Expected username %s, got %s", username, extractedUsername)
	}

	if extractedClaims["email"] != "test@example.com" {
		t.Errorf("Expected email test@example.com, got %v", extractedClaims["email"])
	}
}

func TestVerifyExternalJWT_ExpiredToken(t *testing.T) {
	secret := "test-secret-key"
	algorithm := "HS256"

	// Create an expired JWT token
	claims := jwt.MapClaims{
		"sub": "testuser",
		"exp": time.Now().Add(-time.Hour).Unix(), // Expired 1 hour ago
		"iat": time.Now().Add(-2 * time.Hour).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("Failed to create test token: %v", err)
	}

	// Verify the token - should fail
	_, _, err = auth.VerifyExternalJWT(tokenString, secret, algorithm, "sub")
	if err == nil {
		t.Fatal("VerifyExternalJWT() should fail for expired token")
	}
}

func TestVerifyExternalJWT_InvalidSignature(t *testing.T) {
	secret := "test-secret-key"
	wrongSecret := "wrong-secret-key"
	algorithm := "HS256"

	// Create a token with one secret
	claims := jwt.MapClaims{
		"sub": "testuser",
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("Failed to create test token: %v", err)
	}

	// Try to verify with a different secret - should fail
	_, _, err = auth.VerifyExternalJWT(tokenString, wrongSecret, algorithm, "sub")
	if err == nil {
		t.Fatal("VerifyExternalJWT() should fail for invalid signature")
	}
}

func TestVerifyExternalJWT_MissingUsernameClaim(t *testing.T) {
	secret := "test-secret-key"
	algorithm := "HS256"

	// Create a token without the 'sub' claim
	claims := jwt.MapClaims{
		"email": "test@example.com",
		"exp":   time.Now().Add(time.Hour).Unix(),
		"iat":   time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("Failed to create test token: %v", err)
	}

	// Verify the token - should fail due to missing 'sub' claim
	_, _, err = auth.VerifyExternalJWT(tokenString, secret, algorithm, "sub")
	if err == nil {
		t.Fatal("VerifyExternalJWT() should fail for missing username claim")
	}
}

func TestVerifyExternalJWT_CustomUserIdentifier(t *testing.T) {
	secret := "test-secret-key"
	algorithm := "HS256"
	username := "testuser"

	// Create a token with custom username field
	claims := jwt.MapClaims{
		"username": username,
		"exp":      time.Now().Add(time.Hour).Unix(),
		"iat":      time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("Failed to create test token: %v", err)
	}

	// Verify the token with custom user identifier field
	extractedUsername, _, err := auth.VerifyExternalJWT(tokenString, secret, algorithm, "username")
	if err != nil {
		t.Fatalf("VerifyExternalJWT() failed: %v", err)
	}

	if extractedUsername != username {
		t.Errorf("Expected username %s, got %s", username, extractedUsername)
	}
}

func TestExtractGroupsFromClaims_ArrayOfStrings(t *testing.T) {
	claims := map[string]interface{}{
		"groups": []string{"admin", "users"},
	}

	groups := auth.ExtractGroupsFromClaims(claims, "groups")

	if len(groups) != 2 {
		t.Fatalf("Expected 2 groups, got %d", len(groups))
	}

	if groups[0] != "admin" || groups[1] != "users" {
		t.Errorf("Expected groups [admin, users], got %v", groups)
	}
}

func TestExtractGroupsFromClaims_ArrayOfInterfaces(t *testing.T) {
	claims := map[string]interface{}{
		"groups": []interface{}{"admin", "users"},
	}

	groups := auth.ExtractGroupsFromClaims(claims, "groups")

	if len(groups) != 2 {
		t.Fatalf("Expected 2 groups, got %d", len(groups))
	}

	if groups[0] != "admin" || groups[1] != "users" {
		t.Errorf("Expected groups [admin, users], got %v", groups)
	}
}

func TestExtractGroupsFromClaims_SingleString(t *testing.T) {
	claims := map[string]interface{}{
		"groups": "admin",
	}

	groups := auth.ExtractGroupsFromClaims(claims, "groups")

	if len(groups) != 1 {
		t.Fatalf("Expected 1 group, got %d", len(groups))
	}

	if groups[0] != "admin" {
		t.Errorf("Expected group admin, got %s", groups[0])
	}
}

func TestExtractGroupsFromClaims_MissingClaim(t *testing.T) {
	claims := map[string]interface{}{
		"email": "test@example.com",
	}

	groups := auth.ExtractGroupsFromClaims(claims, "groups")

	if len(groups) != 0 {
		t.Fatalf("Expected 0 groups, got %d", len(groups))
	}
}

func TestExtractGroupsFromClaims_CustomClaimName(t *testing.T) {
	claims := map[string]interface{}{
		"roles": []string{"admin", "editor"},
	}

	groups := auth.ExtractGroupsFromClaims(claims, "roles")

	if len(groups) != 2 {
		t.Fatalf("Expected 2 groups, got %d", len(groups))
	}

	if groups[0] != "admin" || groups[1] != "editor" {
		t.Errorf("Expected groups [admin, editor], got %v", groups)
	}
}

func TestExtractGroupsFromClaims_NestedSingleLevel(t *testing.T) {
	claims := map[string]interface{}{
		"custom": map[string]interface{}{
			"groups": []string{"admin", "users"},
		},
	}

	groups := auth.ExtractGroupsFromClaims(claims, "custom:groups")

	if len(groups) != 2 {
		t.Fatalf("Expected 2 groups, got %d", len(groups))
	}

	if groups[0] != "admin" || groups[1] != "users" {
		t.Errorf("Expected groups [admin, users], got %v", groups)
	}
}

func TestExtractGroupsFromClaims_NestedMultipleLevels(t *testing.T) {
	claims := map[string]interface{}{
		"realm": map[string]interface{}{
			"roles": map[string]interface{}{
				"groups": []string{"superadmin", "developers", "testers"},
			},
		},
	}

	groups := auth.ExtractGroupsFromClaims(claims, "realm:roles:groups")

	if len(groups) != 3 {
		t.Fatalf("Expected 3 groups, got %d", len(groups))
	}

	if groups[0] != "superadmin" || groups[1] != "developers" || groups[2] != "testers" {
		t.Errorf("Expected groups [superadmin, developers, testers], got %v", groups)
	}
}

func TestExtractGroupsFromClaims_NestedArrayOfInterfaces(t *testing.T) {
	claims := map[string]interface{}{
		"resourceAccess": map[string]interface{}{
			"client": []interface{}{"role1", "role2", "role3"},
		},
	}

	groups := auth.ExtractGroupsFromClaims(claims, "resourceAccess:client")

	if len(groups) != 3 {
		t.Fatalf("Expected 3 groups, got %d", len(groups))
	}

	if groups[0] != "role1" || groups[1] != "role2" || groups[2] != "role3" {
		t.Errorf("Expected groups [role1, role2, role3], got %v", groups)
	}
}

func TestExtractGroupsFromClaims_NestedSingleString(t *testing.T) {
	claims := map[string]interface{}{
		"auth": map[string]interface{}{
			"group": "admin",
		},
	}

	groups := auth.ExtractGroupsFromClaims(claims, "auth:group")

	if len(groups) != 1 {
		t.Fatalf("Expected 1 group, got %d", len(groups))
	}

	if groups[0] != "admin" {
		t.Errorf("Expected group admin, got %s", groups[0])
	}
}

func TestExtractGroupsFromClaims_NestedInvalidPath(t *testing.T) {
	claims := map[string]interface{}{
		"custom": map[string]interface{}{
			"roles": []string{"admin"},
		},
	}

	// Try to access non-existent nested path - should fallback and return empty
	groups := auth.ExtractGroupsFromClaims(claims, "custom:nonexistent:groups")

	if len(groups) != 0 {
		t.Fatalf("Expected 0 groups for invalid nested path, got %d", len(groups))
	}
}

func TestExtractGroupsFromClaims_NestedFallbackToDirect(t *testing.T) {
	claims := map[string]interface{}{
		"groups": []string{"direct_admin"},
		"custom": map[string]interface{}{
			"roles": []string{"nested_admin"},
		},
	}

	// When nested path fails, should fallback to direct lookup
	groups := auth.ExtractGroupsFromClaims(claims, "custom:nonexistent")

	// Since "custom:nonexistent" path fails, should fallback but the field doesn't exist
	if len(groups) != 0 {
		t.Fatalf("Expected 0 groups for invalid nested path without fallback match, got %d", len(groups))
	}
}

func TestExtractGroupsFromClaims_DeepNesting(t *testing.T) {
	claims := map[string]interface{}{
		"keycloak": map[string]interface{}{
			"realm": map[string]interface{}{
				"roles": map[string]interface{}{
					"app": map[string]interface{}{
						"groups": []string{"admin", "user", "guest"},
					},
				},
			},
		},
	}

	groups := auth.ExtractGroupsFromClaims(claims, "keycloak:realm:roles:app:groups")

	if len(groups) != 3 {
		t.Fatalf("Expected 3 groups, got %d", len(groups))
	}

	if groups[0] != "admin" || groups[1] != "user" || groups[2] != "guest" {
		t.Errorf("Expected groups [admin, user, guest], got %v", groups)
	}
}
