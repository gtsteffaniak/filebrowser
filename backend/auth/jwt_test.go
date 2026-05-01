package auth_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
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

func TestVerifyExternalJWT_ES256_PublicKeyPEM(t *testing.T) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	pubDER, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatalf("MarshalPKIXPublicKey: %v", err)
	}
	pubPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}))

	username := "ec-user"
	claims := jwt.MapClaims{
		"sub": username,
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	tokenString, err := token.SignedString(priv)
	if err != nil {
		t.Fatalf("SignedString: %v", err)
	}

	got, _, err := auth.VerifyExternalJWT(tokenString, pubPEM, "ES256", "sub")
	if err != nil {
		t.Fatalf("VerifyExternalJWT: %v", err)
	}
	if got != username {
		t.Errorf("username: got %q want %q", got, username)
	}
}

func TestVerifyExternalJWT_RS256_PublicKeyPEM(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	pubDER, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatalf("MarshalPKIXPublicKey: %v", err)
	}
	pubPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}))

	username := "rsa-user"
	claims := jwt.MapClaims{
		"sub": username,
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, err := token.SignedString(priv)
	if err != nil {
		t.Fatalf("SignedString: %v", err)
	}

	got, _, err := auth.VerifyExternalJWT(tokenString, pubPEM, "RS256", "sub")
	if err != nil {
		t.Fatalf("VerifyExternalJWT: %v", err)
	}
	if got != username {
		t.Errorf("username: got %q want %q", got, username)
	}
}

func TestVerifyExternalJWT_ES256_WrongCurveRejected(t *testing.T) {
	p256Priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	p384Priv, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	verifyDER, err := x509.MarshalPKIXPublicKey(&p384Priv.PublicKey)
	if err != nil {
		t.Fatalf("MarshalPKIXPublicKey: %v", err)
	}
	verifyPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: verifyDER}))

	claims := jwt.MapClaims{
		"sub": "u",
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	tokenString, err := token.SignedString(p256Priv)
	if err != nil {
		t.Fatalf("SignedString: %v", err)
	}

	_, _, err = auth.VerifyExternalJWT(tokenString, verifyPEM, "ES256", "sub")
	if err == nil {
		t.Fatal("expected error when PEM public key curve does not match algorithm")
	}
}

func TestVerifyExternalJWT_CertificatePEM(t *testing.T) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "jwt-test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}
	certDER, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("CreateCertificate: %v", err)
	}
	certPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}))

	username := "cert-user"
	claims := jwt.MapClaims{
		"sub": username,
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	tokenString, err := token.SignedString(priv)
	if err != nil {
		t.Fatalf("SignedString: %v", err)
	}

	got, _, err := auth.VerifyExternalJWT(tokenString, certPEM, "ES256", "sub")
	if err != nil {
		t.Fatalf("VerifyExternalJWT: %v", err)
	}
	if got != username {
		t.Errorf("username: got %q want %q", got, username)
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
