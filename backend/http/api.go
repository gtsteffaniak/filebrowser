package http

import (
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/gtsteffaniak/filebrowser/users"
)

func createApiHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	// get request body like:

	create
	return renderJSON(w, r, response)
}

func makeSignedTokenAPI(user *users.User, duration time.Duration) (string, error) {
	claims := &authToken{
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(duration)),
			Issuer:    user.Username,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(config.Auth.Key)
}
