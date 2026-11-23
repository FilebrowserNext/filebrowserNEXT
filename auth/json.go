package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"

	"github.com/filebrowser/filebrowser/v2/settings"
	"github.com/filebrowser/filebrowser/v2/users"
)

// MethodJSONAuth is used to identify json auth.
const MethodJSONAuth settings.AuthMethod = "json"

// dummyHash is used to prevent user enumeration timing attacks.
// It MUST be a valid bcrypt hash.
const dummyHash = "$2a$10$O4mEMeOL/nit6zqe.WQXauLRbRlzb3IgLHsa26Pf0N/GiU9b.wK1m"

type jsonCred struct {
	Password string `json:"password"`
	Username string `json:"username"`
	Captcha  string `json:"captcha"`
}

// JSONAuth is a json implementation of an Auther.
type JSONAuth struct {
	Altcha *Altcha `json:"altcha,omitempty" yaml:"altcha,omitempty"`
	// ReCaptcha is the former Google reCAPTCHA configuration. It is no longer
	// honored and is only decoded so that operators can be warned about it.
	ReCaptcha *LegacyReCaptcha `json:"recaptcha,omitempty" yaml:"recaptcha,omitempty"`
}

// LegacyReCaptcha mirrors the removed Google reCAPTCHA settings.
type LegacyReCaptcha struct {
	Host   string `json:"host"`
	Key    string `json:"key"`
	Secret string `json:"secret"`
}

// LegacyReCaptchaConfigured reports whether a stored configuration still
// references the removed Google reCAPTCHA integration.
func (a JSONAuth) LegacyReCaptchaConfigured() bool {
	return a.ReCaptcha != nil && (a.ReCaptcha.Key != "" || a.ReCaptcha.Secret != "")
}

// VerifyCaptcha checks the captcha payload when a captcha is configured.
func (a JSONAuth) VerifyCaptcha(payload string) error {
	if !a.Altcha.Enabled() {
		return nil
	}
	return a.Altcha.Verify(payload)
}

// Auth authenticates the user via a json in content body.
func (a JSONAuth) Auth(r *http.Request, usr users.Store, _ *settings.Settings, srv *settings.Server) (*users.User, error) {
	var cred jsonCred

	if r.Body == nil {
		return nil, os.ErrPermission
	}

	err := json.NewDecoder(r.Body).Decode(&cred)
	if err != nil {
		return nil, os.ErrPermission
	}

	if err := a.VerifyCaptcha(cred.Captcha); err != nil {
		if errors.Is(err, ErrCaptchaRequired) {
			return nil, os.ErrPermission
		}
		return nil, err
	}

	u, err := usr.Get(srv.Root, srv.FollowExternalSymlinks, cred.Username)

	hash := dummyHash
	if err == nil {
		hash = u.Password
	}

	if !users.CheckPwd(cred.Password, hash) {
		return nil, os.ErrPermission
	}

	if err != nil {
		return nil, os.ErrPermission
	}

	return u, nil
}

// LoginPage tells that json auth doesn't require a login page.
func (a JSONAuth) LoginPage() bool {
	return true
}
