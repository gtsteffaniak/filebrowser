package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// Simple JWT generator without external dependencies
func main() {
	// Secret key - must match the one in your config file
	secret := "wrong-key"

	// Check if custom secret provided via command line
	if len(os.Args) > 1 {
		secret = os.Args[1]
	}

	// Create token claims with 10 year expiration
	now := time.Now()
	expiresAt := now.Add(10 * 365 * 24 * time.Hour) // 10 years

	header := map[string]interface{}{
		"alg": "HS256",
		"typ": "JWT",
	}

	claims := map[string]interface{}{
		"sub":    "testadmin",             // username (subject)
		"email":  "testadmin@example.com", // email claim
		"name":   "Test Admin",            // full name
		"groups": []string{"admin"},       // groups - includes admin for testing
		"iat":    now.Unix(),              // issued at
		"exp":    expiresAt.Unix(),        // expires at
		"nbf":    now.Unix(),              // not before
	}

	// Encode header
	headerJSON, err := json.Marshal(header)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding header: %v\n", err)
		os.Exit(1)
	}
	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)

	// Encode claims
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding claims: %v\n", err)
		os.Exit(1)
	}
	claimsB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)

	// Create signature
	message := headerB64 + "." + claimsB64
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(message))
	signature := base64.RawURLEncoding.EncodeToString(h.Sum(nil))

	// Combine to form complete JWT
	tokenString := message + "." + signature

	// Output the token
	fmt.Println("=== JWT Token Generated ===")
	fmt.Println()
	fmt.Println("Token:")
	fmt.Println(tokenString)
	fmt.Println()
	// Verify token can be decoded
	parts := strings.Split(tokenString, ".")
	if len(parts) == 3 {
		fmt.Println("✓ Token structure is valid (3 parts)")
	} else {
		fmt.Fprintf(os.Stderr, "✗ Invalid token structure\n")
		os.Exit(1)
	}

	// Save to file for easy access
	filename := "test-jwt-token.txt"
	if err := os.WriteFile(filename, []byte(tokenString), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not write token to file: %v\n", err)
	} else {
		fmt.Printf("✓ Token saved to: %s\n", filename)
		fmt.Println()
	}

	// Also save to shell script for easy sourcing
	scriptContent := fmt.Sprintf("#!/bin/bash\n# Source this file to set JWT token as environment variable\n# Usage: source jwt-env.sh\nexport TEST_JWT_TOKEN=\"%s\"\necho \"JWT token loaded into TEST_JWT_TOKEN\"\n", tokenString)
	if err := os.WriteFile("jwt-env.sh", []byte(scriptContent), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not write shell script: %v\n", err)
	} else {
		fmt.Println("✓ Shell script saved to: jwt-env.sh")
		fmt.Println("  Usage: source jwt-env.sh")
		fmt.Println()
	}
}
