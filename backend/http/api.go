package http

import (
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/gtsteffaniak/filebrowser/users"
)

func createApiHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	// get request body like:

	token, err := makeSignedTokenAPI(d.user, time.Minute, users.Permissions{})
	if err != nil {
		return 500, err
	}
	response := HttpResponse{
		Message: "here is your token!",
		Token:   token,
	}
	return renderJSON(w, r, response)
}

func makeSignedTokenAPI(user *users.User, duration time.Duration, perms users.Permissions) (string, error) {
	claims := &authToken{
		User: users.User{
			Username: user.Username,
			ID:       user.ID,
			Perm:     perms,
		},
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(duration)),
			Issuer:    user.Username,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(config.Auth.Key)
}
