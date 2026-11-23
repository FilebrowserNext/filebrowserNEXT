package fbhttp

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/filebrowser/filebrowser/v2/auth"
	"github.com/filebrowser/filebrowser/v2/storage"
)

// captchaLimiter bounds how many challenges a client may request. Creating a
// challenge is cheap, but each one occupies the replay cache once solved.
var captchaLimiter = newRateLimiter(30, 5*time.Minute)

// jsonAuther returns the stored JSON auther, or nil when another
// authentication method is active.
func jsonAuther(d *data) (*auth.JSONAuth, error) {
	if d.settings.AuthMethod != auth.MethodJSONAuth {
		return nil, nil
	}
	raw, err := d.store.Auth.Get(auth.MethodJSONAuth)
	if err != nil {
		return nil, err
	}
	jsonAuth, ok := raw.(*auth.JSONAuth)
	if !ok {
		return nil, fmt.Errorf("unexpected auther type %T", raw)
	}
	return jsonAuth, nil
}

// captchaChallengeHandler issues a signed ALTCHA proof-of-work challenge.
var captchaChallengeHandler = func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	if !captchaLimiter.allow(clientIP(r)) {
		return http.StatusTooManyRequests, nil
	}
	jsonAuth, err := jsonAuther(d)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	if jsonAuth == nil || !jsonAuth.Altcha.Enabled() {
		return http.StatusNotFound, nil
	}
	challenge, err := jsonAuth.Altcha.CreateChallenge()
	if err != nil {
		return http.StatusInternalServerError, err
	}
	w.Header().Set("Cache-Control", "no-store")
	return renderJSON(w, r, challenge)
}

// verifySignupCaptcha checks the captcha sent along a signup request.
func verifySignupCaptcha(d *data, payload string) (int, error) {
	jsonAuth, err := jsonAuther(d)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	if jsonAuth == nil {
		return 0, nil
	}
	err = jsonAuth.VerifyCaptcha(payload)
	switch {
	case errors.Is(err, auth.ErrCaptchaRequired):
		return http.StatusForbidden, nil
	case err != nil:
		return http.StatusInternalServerError, err
	}
	return 0, nil
}

// warnLegacyCaptcha tells operators that a stored Google reCAPTCHA
// configuration is no longer enforced.
func warnLegacyCaptcha(store *storage.Storage) {
	set, err := store.Settings.Get()
	if err != nil || set.AuthMethod != auth.MethodJSONAuth {
		return
	}
	raw, err := store.Auth.Get(auth.MethodJSONAuth)
	if err != nil {
		log.Printf("captcha: cannot read authentication settings: %v", err)
		return
	}
	jsonAuth, ok := raw.(*auth.JSONAuth)
	if !ok || !jsonAuth.LegacyReCaptchaConfigured() || jsonAuth.Altcha.Enabled() {
		return
	}
	log.Printf("WARNING: Google reCAPTCHA support was removed and the stored reCAPTCHA settings are ignored. " +
		"Run 'filebrowser config set --altcha' to enable the self-hosted ALTCHA captcha.")
}
