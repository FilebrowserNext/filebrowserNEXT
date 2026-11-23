package fbhttp

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	altcha "github.com/altcha-org/altcha-lib-go/v2"
	"github.com/asdine/storm/v3"

	fbAuth "github.com/filebrowser/filebrowser/v2/auth"
	"github.com/filebrowser/filebrowser/v2/settings"
	"github.com/filebrowser/filebrowser/v2/storage"
	"github.com/filebrowser/filebrowser/v2/storage/bolt"
)

func captchaStorage(t *testing.T, captcha *fbAuth.Altcha) *storage.Storage {
	t.Helper()
	db, err := storm.Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	st, err := bolt.NewStorage(db)
	if err != nil {
		t.Fatalf("failed to get storage: %v", err)
	}
	if err := st.Settings.Save(&settings.Settings{
		Key:                   []byte("test-signing-key"),
		Signup:                true,
		AuthMethod:            fbAuth.MethodJSONAuth,
		MinimumPasswordLength: 1,
	}); err != nil {
		t.Fatalf("failed to save settings: %v", err)
	}
	if err := st.Auth.Save(&fbAuth.JSONAuth{Altcha: captcha}); err != nil {
		t.Fatalf("failed to save auther: %v", err)
	}
	return st
}

func fetchChallenge(t *testing.T, st *storage.Storage, server *settings.Server) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/captcha/challenge", http.NoBody)
	rec := httptest.NewRecorder()
	handle(captchaChallengeHandler, "", st, server).ServeHTTP(rec, req)
	return rec
}

func solvedCaptcha(t *testing.T, body []byte) string {
	t.Helper()
	var challenge altcha.Challenge
	if err := json.Unmarshal(body, &challenge); err != nil {
		t.Fatalf("decode challenge: %v", err)
	}
	solution, err := altcha.SolveChallenge(altcha.SolveChallengeOptions{
		Challenge: challenge,
		DeriveKey: altcha.DeriveKeySHA(),
	})
	if err != nil || solution == nil {
		t.Fatalf("solve challenge: solution=%v err=%v", solution, err)
	}
	raw, err := json.Marshal(altcha.Payload{Challenge: challenge, Solution: *solution})
	if err != nil {
		t.Fatalf("encode payload: %v", err)
	}
	return base64.StdEncoding.EncodeToString(raw)
}

func TestCaptchaChallengeNotFoundWhenDisabled(t *testing.T) {
	st := captchaStorage(t, nil)
	rec := fetchChallenge(t, st, &settings.Server{Root: t.TempDir()})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 without ALTCHA, got %d", rec.Code)
	}
}

// The challenge endpoint must never expose the signing secret, and signup
// must require a fresh, solved challenge once ALTCHA is enabled.
func TestSignupRequiresSolvedCaptcha(t *testing.T) {
	secret, err := fbAuth.NewAltchaSecret()
	if err != nil {
		t.Fatalf("NewAltchaSecret: %v", err)
	}
	st := captchaStorage(t, &fbAuth.Altcha{Secret: secret, Complexity: fbAuth.MinAltchaComplexity})
	server := &settings.Server{Root: t.TempDir()}

	signup := func(username, captcha string) int {
		body := `{"username":"` + username + `","password":"CaptchaPw12345!","captcha":"` + captcha + `"}`
		req := httptest.NewRequest(http.MethodPost, "/api/signup", strings.NewReader(body))
		rec := httptest.NewRecorder()
		handle(signupHandler, "", st, server).ServeHTTP(rec, req)
		return rec.Code
	}

	if code := signup("bot", ""); code != http.StatusForbidden {
		t.Fatalf("signup without captcha: expected 403, got %d", code)
	}

	rec := fetchChallenge(t, st, server)
	if rec.Code != http.StatusOK {
		t.Fatalf("challenge: expected 200, got %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), secret) {
		t.Fatal("challenge response leaks the signing secret")
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("challenge must not be cached, Cache-Control=%q", rec.Header().Get("Cache-Control"))
	}

	payload := solvedCaptcha(t, rec.Body.Bytes())
	if code := signup("human", payload); code != http.StatusOK {
		t.Fatalf("signup with solved captcha: expected 200, got %d", code)
	}
	if code := signup("replay", payload); code != http.StatusForbidden {
		t.Fatalf("signup replaying a captcha: expected 403, got %d", code)
	}
}
