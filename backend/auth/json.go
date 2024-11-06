package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/gtsteffaniak/filebrowser/settings"
	"github.com/gtsteffaniak/filebrowser/users"
)

type jsonCred struct {
	Password  string `json:"password"`
	Username  string `json:"username"`
	ReCaptcha string `json:"recaptcha"`
}

// JSONAuth is a json implementation of an Auther.
type JSONAuth struct {
	ReCaptcha *ReCaptcha `json:"recaptcha" yaml:"recaptcha"`
}

// Auth authenticates the user via a json in content body.
func (a JSONAuth) Auth(r *http.Request, userStore *users.Storage) (*users.User, error) {
	config := &settings.Config
	var cred jsonCred

	if r.Body == nil {
		fmt.Println("nil body")

		return nil, os.ErrPermission
	}
	err := json.NewDecoder(r.Body).Decode(&cred)
	if err != nil {
		fmt.Println("error decoding body")
		return nil, os.ErrPermission
	}

	// If ReCaptcha is enabled, check the code.
	if a.ReCaptcha != nil && len(a.ReCaptcha.Secret) > 0 {
		ok, err := a.ReCaptcha.Ok(cred.ReCaptcha) //nolint:govet

		if err != nil {
			return nil, err
		}

		if !ok {
			fmt.Println("error other")
			return nil, os.ErrPermission
		}
	}
	fmt.Println("cred.Username: ", cred.Password)
	u, err := userStore.Get(config.Server.Root, cred.Username)
	fmt.Println("u: ", u.Password, u.Username)
	if err != nil || !users.CheckPwd(cred.Password, u.Password) {
		fmt.Println("error check password")
		return nil, os.ErrPermission
	}

	return u, nil
}

// LoginPage tells that json auth doesn't require a login page.
func (a JSONAuth) LoginPage() bool {
	return true
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
