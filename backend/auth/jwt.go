package auth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"

	"github.com/golang-jwt/jwt/v4"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/go-logger/logger"
)

// MethodJwtAuth is used to identify JWT external authentication.
const MethodJwtAuth = "jwt"

// JwtAuth is an external JWT implementation of an auther.
// It verifies JWT tokens signed with a shared secret and extracts user identity.
type JwtAuth struct {
	Secret    string `json:"secret"`    // shared secret for verifying JWT signatures
	Algorithm string `json:"algorithm"` // JWT signing algorithm (HS256, HS384, HS512, RS256, ES256, etc.)
}

// ExternalJwtClaims represents the expected claims in an external JWT token
type ExternalJwtClaims struct {
	jwt.RegisteredClaims
	Username string                 `json:"sub"`                    // subject claim (username)
	Email    string                 `json:"email,omitempty"`        // email claim
	Name     string                 `json:"name,omitempty"`         // full name claim
	Groups   []string               `json:"groups,omitempty"`       // groups claim
	Custom   map[string]interface{} `json:"-"`                      // catch-all for custom claims
}

// Auth authenticates the user via an external JWT token.
// The token is verified using the configured secret and algorithm.
func (a JwtAuth) Auth(r *http.Request, usr *users.Storage) (*users.User, error) {
	// This should not be called directly - JWT auth is handled in middleware
	// because we need access to the full config (header name, user identifier field, etc.)
	return nil, fmt.Errorf("JWT auth must be handled in middleware")
}

// VerifyExternalJWT verifies an external JWT token and extracts the username claim
func VerifyExternalJWT(tokenString string, secret string, algorithm string, userIdentifierField string) (string, map[string]interface{}, error) {
	// Parse the token with the secret
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Verify the signing method matches what we expect
		expectedMethod := jwt.GetSigningMethod(algorithm)
		if expectedMethod == nil {
			return nil, fmt.Errorf("unsupported signing algorithm: %s", algorithm)
		}

		if token.Method.Alg() != expectedMethod.Alg() {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		return jwtVerificationKey(secret, token.Method, algorithm)
	})

	if err != nil {
		logger.Debugf("JWT token verification failed: %v", err)
		return "", nil, fmt.Errorf("invalid JWT token: %w", err)
	}

	if !token.Valid {
		return "", nil, fmt.Errorf("JWT token is invalid")
	}

	// Extract claims
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", nil, fmt.Errorf("failed to parse JWT claims")
	}

	// Check if token is expired
	if err := claims.Valid(); err != nil {
		logger.Debugf("JWT claims validation failed: %v", err)
		return "", nil, fmt.Errorf("JWT token is expired or invalid: %w", err)
	}

	// Extract username from the configured field (default: "sub")
	var username string
	if userVal, ok := claims[userIdentifierField]; ok {
		if userStr, ok := userVal.(string); ok {
			username = userStr
		}
	}

	if username == "" {
		return "", nil, fmt.Errorf("JWT token missing required claim: %s", userIdentifierField)
	}

	// Convert claims to map for additional processing
	claimsMap := make(map[string]interface{})
	for k, v := range claims {
		claimsMap[k] = v
	}

	logger.Debugf("Successfully verified JWT token for user: %s", username)
	return username, claimsMap, nil
}

func jwtVerificationKey(secret string, method jwt.SigningMethod, algorithm string) (interface{}, error) {
	switch method.(type) {
	case *jwt.SigningMethodHMAC:
		return []byte(secret), nil
	case *jwt.SigningMethodRSA:
		key, err := parsePublicKeyFromPEM(secret)
		if err != nil {
			return nil, err
		}
		rsaKey, ok := key.(*rsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("JWT secret must contain an RSA public key in PEM format for algorithm %s", algorithm)
		}
		return rsaKey, nil
	case *jwt.SigningMethodECDSA:
		key, err := parsePublicKeyFromPEM(secret)
		if err != nil {
			return nil, err
		}
		ecKey, ok := key.(*ecdsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("JWT secret must contain an ECDSA public key in PEM format for algorithm %s", algorithm)
		}
		if err := ecdsaCurveMatchesAlgorithm(algorithm, ecKey); err != nil {
			return nil, err
		}
		return ecKey, nil
	default:
		return nil, fmt.Errorf("unsupported JWT signing method: %s", method.Alg())
	}
}

func ecdsaCurveMatchesAlgorithm(algorithm string, pub *ecdsa.PublicKey) error {
	var want elliptic.Curve
	switch algorithm {
	case "ES256":
		want = elliptic.P256()
	case "ES384":
		want = elliptic.P384()
	case "ES512":
		want = elliptic.P521()
	default:
		return nil
	}
	if pub.Curve != want {
		return fmt.Errorf("ECDSA public key curve does not match algorithm %s", algorithm)
	}
	return nil
}

func parsePublicKeyFromPEM(pemData string) (interface{}, error) {
	rest := []byte(pemData)
	var lastErr error
	for len(rest) > 0 {
		block, rem := pem.Decode(rest)
		if block == nil {
			break
		}
		rest = rem

		var key interface{}
		var err error
		switch block.Type {
		case "CERTIFICATE":
			var cert *x509.Certificate
			cert, err = x509.ParseCertificate(block.Bytes)
			if err == nil {
				key = cert.PublicKey
			}
		case "PUBLIC KEY":
			key, err = x509.ParsePKIXPublicKey(block.Bytes)
		case "RSA PUBLIC KEY":
			key, err = x509.ParsePKCS1PublicKey(block.Bytes)
		case "EC PUBLIC KEY":
			key, err = x509.ParsePKIXPublicKey(block.Bytes)
		default:
			continue
		}
		if err != nil {
			lastErr = err
			continue
		}
		if key != nil {
			return key, nil
		}
	}
	if lastErr != nil {
		return nil, fmt.Errorf("failed to parse JWT public key from PEM: %w", lastErr)
	}
	return nil, fmt.Errorf("no public key found in JWT secret (expected PEM: CERTIFICATE, PUBLIC KEY, RSA PUBLIC KEY, or EC PUBLIC KEY)")
}

// ExtractGroupsFromClaims extracts groups from JWT claims based on the configured groups claim field
func ExtractGroupsFromClaims(claims map[string]interface{}, groupsClaimField string) []string {
	var groups []string

	if groupsVal, ok := claims[groupsClaimField]; ok {
		switch v := groupsVal.(type) {
		case []interface{}:
			// Groups as array of interfaces
			for _, g := range v {
				if groupStr, ok := g.(string); ok {
					groups = append(groups, groupStr)
				}
			}
		case []string:
			// Groups as array of strings
			groups = v
		case string:
			// Single group as string
			groups = []string{v}
		default:
			// Try to unmarshal as JSON array
			if jsonBytes, err := json.Marshal(v); err == nil {
				var groupsArray []string
				if err := json.Unmarshal(jsonBytes, &groupsArray); err == nil {
					groups = groupsArray
				}
			}
		}
	}

	return groups
}
