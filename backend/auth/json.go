package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/common/errors"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
)

// JSONAuth is a json implementation of an Auther.
type JSONAuth struct {
	ReCaptcha *ReCaptcha `json:"recaptcha" yaml:"recaptcha"`
}

// Auth authenticates the user via a json in content body.
func (auther JSONAuth) Auth(r *http.Request, userStore *users.Storage) (*users.User, error) {
	username := r.URL.Query().Get("username")
	recaptcha := r.URL.Query().Get("recaptcha")
	password := r.Header.Get("X-Password")
	// URL-decode password to support special characters in headers
	password, err := url.QueryUnescape(password)
	if err != nil {
		return nil, fmt.Errorf("invalid password encoding: %v", err)
	}
	totpCode := r.Header.Get("X-Secret")

	// If ReCaptcha is enabled, check the code.
	if auther.ReCaptcha != nil && len(auther.ReCaptcha.Secret) > 0 {
		ok, err := auther.ReCaptcha.Ok(recaptcha) //nolint:govet

		if err != nil {
			return nil, err
		}

		if !ok {
			return nil, os.ErrPermission
		}
	}

	user, err := userStore.Get(username)
	if err != nil {
		return nil, fmt.Errorf("unable to get user from store: %v", err)
	}
	err = users.CheckPwd(password, user.Password)
	if err != nil {
		return nil, err
	}

	// check for OTP for password
	if user.TOTPSecret != "" {
		if totpCode == "" {
			return nil, errors.ErrNoTotpProvided
		}
		err = VerifyTotpCode(user, totpCode, userStore)
		if err != nil {
			return nil, err
		}
	}

	if user.LoginMethod != users.LoginMethodPassword {
		return nil, errors.ErrWrongLoginMethod
	}

	return user, nil

}

const reCaptchaAPI = "/recaptcha/api/siteverify"

// ReCaptcha identifies a recaptcha connection.
type ReCaptcha struct {
	Host   string `json:"host"`
	Key    string `json:"key"`
	Secret string `json:"secret"`
}

// Ok checks if a reCaptcha responde is correct.
func (r *ReCaptcha) Ok(response string) (bool, error) {
	body := url.Values{}
	body.Set("secret", r.Secret)
	body.Add("response", response)

	client := &http.Client{}

	resp, err := client.Post(
		r.Host+reCaptchaAPI,
		"application/x-www-form-urlencoded",
		strings.NewReader(body.Encode()),
	)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, nil
	}

	var data struct {
		Success bool `json:"success"`
	}

	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		return false, err
	}

	return data.Success, nil
}
