package fbhttp

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/asdine/storm/v3"
	"github.com/golang-jwt/jwt/v5"

	fbAuth "github.com/filebrowser/filebrowser/v2/auth"
	"github.com/filebrowser/filebrowser/v2/settings"
	"github.com/filebrowser/filebrowser/v2/storage/bolt"
	"github.com/filebrowser/filebrowser/v2/users"
)

// Regression for the username-normalization home-directory collision
// (GHSA-7rc3-g7h6-22m7): with Signup and CreateUserDir enabled, two distinct
// usernames that cleanUsername() normalizes to the same directory must not be
// handed the same home directory. The second registration is rejected.
func TestSignupRejectsCollidingNormalizedScope(t *testing.T) {
	root := t.TempDir()

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
		CreateUserDir:         true,
		UserHomeBasePath:      "/users",
		MinimumPasswordLength: 1,
	}); err != nil {
		t.Fatalf("failed to save settings: %v", err)
	}

	server := &settings.Server{Root: root}

	signup := func(username string) *httptest.ResponseRecorder {
		body := `{"username":"` + username + `","password":"CollidePw12345!"}`
		req, _ := http.NewRequest(http.MethodPost, "/signup", strings.NewReader(body))
		rec := httptest.NewRecorder()
		handle(signupHandler, "", st, server).ServeHTTP(rec, req)
		return rec
	}

	// Victim registers first and gets /users/teamone-x.
	if rec := signup("teamone-x"); rec.Code != http.StatusOK {
		t.Fatalf("first signup: expected 200, got %d body=%q", rec.Code, rec.Body.String())
	}

	// Attacker picks a distinct username that normalizes to the same scope.
	if rec := signup("teamone/x"); rec.Code != http.StatusConflict {
		t.Fatalf("VULNERABLE: colliding signup expected 409, got %d body=%q", rec.Code, rec.Body.String())
	}

	// The shared scope must still be owned solely by the first user.
	owner, err := st.Users.GetByScope("/users/teamone-x")
	if err != nil {
		t.Fatalf("expected first user to own the scope: %v", err)
	}
	if owner.Username != "teamone-x" {
		t.Fatalf("scope owner = %q, want teamone-x", owner.Username)
	}
}

// Regression for GHSA-v3jv-rmh2-635j: under proxy auth with a non-default
// logout page the JWT expiration is waived, because the proxy owns the session
// lifetime. That exception used to apply to every route on the strength of the
// token alone, so a token stolen before it expired kept working — and could be
// renewed — indefinitely. The proxy must still assert the same identity.
func TestExpiredTokenNeedsProxyAssertion(t *testing.T) {
	const proxyHeader = "X-Fb-User"

	key := []byte("test-signing-key")
	perm := users.Permissions{Download: true}
	st := scopedUserStorage(t, t.TempDir(), perm, key)

	if err := st.Settings.Save(&settings.Settings{
		Key:        key,
		AuthMethod: fbAuth.MethodProxyAuth,
		LogoutPage: "/logged-out",
	}); err != nil {
		t.Fatalf("failed to save settings: %v", err)
	}
	if err := st.Auth.Save(&fbAuth.ProxyAuth{Header: proxyHeader}); err != nil {
		t.Fatalf("failed to save auther: %v", err)
	}

	expired := &authToken{
		User: userInfo{ID: 1, Username: "u", Perm: perm},
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
		},
	}
	expiredToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, expired).SignedString(key)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	protected := withUser(func(w http.ResponseWriter, _ *http.Request, _ *data) (int, error) {
		_, writeErr := w.Write([]byte("protected"))
		return 0, writeErr
	})

	get := func(token, proxyUser string) *httptest.ResponseRecorder {
		req, _ := http.NewRequest(http.MethodGet, "/", http.NoBody)
		req.Header.Set("X-Auth", token)
		if proxyUser != "" {
			req.Header.Set(proxyHeader, proxyUser)
		}
		rec := httptest.NewRecorder()
		handle(protected, "", st, &settings.Server{}).ServeHTTP(rec, req)
		return rec
	}

	t.Run("expired token alone is rejected", func(t *testing.T) {
		if rec := get(expiredToken, ""); rec.Code != http.StatusUnauthorized {
			t.Errorf("VULNERABLE: expired token without the proxy header = %d, body=%q; want 401", rec.Code, rec.Body.String())
		}
	})

	t.Run("expired token for another identity is rejected", func(t *testing.T) {
		if rec := get(expiredToken, "someone-else"); rec.Code != http.StatusUnauthorized {
			t.Errorf("VULNERABLE: expired token with a foreign proxy identity = %d; want 401", rec.Code)
		}
	})

	t.Run("expired token the proxy still asserts is accepted", func(t *testing.T) {
		if rec := get(expiredToken, "u"); rec.Code != http.StatusOK {
			t.Errorf("expired token asserted by the proxy = %d, body=%q; want 200", rec.Code, rec.Body.String())
		}
	})

	t.Run("valid token needs no assertion", func(t *testing.T) {
		if rec := get(signToken(t, perm, key), ""); rec.Code != http.StatusOK {
			t.Errorf("valid token = %d, body=%q; want 200", rec.Code, rec.Body.String())
		}
	})
}

func TestSessionRevocationOnLogoutAndRenew(t *testing.T) {
	key := []byte("test-signing-key")
	perm := users.Permissions{Download: true}
	st := scopedUserStorage(t, t.TempDir(), perm, key)
	server := &settings.Server{}

	protected := withUser(func(w http.ResponseWriter, _ *http.Request, _ *data) (int, error) {
		_, writeErr := w.Write([]byte("ok"))
		return 0, writeErr
	})

	// Login
	user, err := st.Users.Get("", false, uint(1))
	if err != nil {
		t.Fatalf("failed to get user: %v", err)
	}

	loginRec := httptest.NewRecorder()
	loginReq, _ := http.NewRequest(http.MethodGet, "/login", http.NoBody)
	_, _ = printToken(loginRec, loginReq, &data{settings: &settings.Settings{Key: key}}, user, time.Hour)
	token1 := loginRec.Body.String()

	// Verify token1 works
	req, _ := http.NewRequest(http.MethodGet, "/", http.NoBody)
	req.Header.Set("X-Auth", token1)
	rec := httptest.NewRecorder()
	handle(protected, "", st, server).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("token1 expected 200, got %d", rec.Code)
	}

	// Renew token1 -> get token2
	renewReq, _ := http.NewRequest(http.MethodPost, "/renew", http.NoBody)
	renewReq.Header.Set("X-Auth", token1)
	renewRec := httptest.NewRecorder()
	handle(renewHandler(time.Hour), "", st, server).ServeHTTP(renewRec, renewReq)
	if renewRec.Code != http.StatusOK {
		t.Fatalf("renew expected 200, got %d", renewRec.Code)
	}
	token2 := renewRec.Body.String()

	// Verify token1 is now REVOKED and rejected with 401
	reqOld, _ := http.NewRequest(http.MethodGet, "/", http.NoBody)
	reqOld.Header.Set("X-Auth", token1)
	recOld := httptest.NewRecorder()
	handle(protected, "", st, server).ServeHTTP(recOld, reqOld)
	if recOld.Code != http.StatusUnauthorized {
		t.Fatalf("reused token1 expected 401, got %d", recOld.Code)
	}

	// Verify token2 works
	req2, _ := http.NewRequest(http.MethodGet, "/", http.NoBody)
	req2.Header.Set("X-Auth", token2)
	rec2 := httptest.NewRecorder()
	handle(protected, "", st, server).ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("token2 expected 200, got %d", rec2.Code)
	}

	// Logout token2
	logoutReq, _ := http.NewRequest(http.MethodPost, "/logout", http.NoBody)
	logoutReq.Header.Set("X-Auth", token2)
	logoutRec := httptest.NewRecorder()
	handle(logoutHandler, "", st, server).ServeHTTP(logoutRec, logoutReq)
	if logoutRec.Code != http.StatusOK {
		t.Fatalf("logout expected 200, got %d", logoutRec.Code)
	}

	// Verify token2 is now REVOKED and rejected with 401
	reqLoggedOut, _ := http.NewRequest(http.MethodGet, "/", http.NoBody)
	reqLoggedOut.Header.Set("X-Auth", token2)
	recLoggedOut := httptest.NewRecorder()
	handle(protected, "", st, server).ServeHTTP(recLoggedOut, reqLoggedOut)
	if recLoggedOut.Code != http.StatusUnauthorized {
		t.Fatalf("logged out token2 expected 401, got %d", recLoggedOut.Code)
	}
}

func TestTokenInvalidationOnUserUpdate(t *testing.T) {
	key := []byte("test-signing-key")
	perm := users.Permissions{Download: true}
	st := scopedUserStorage(t, t.TempDir(), perm, key)
	server := &settings.Server{}

	protected := withUser(func(w http.ResponseWriter, _ *http.Request, _ *data) (int, error) {
		_, writeErr := w.Write([]byte("ok"))
		return 0, writeErr
	})

	user, err := st.Users.Get("", false, uint(1))
	if err != nil {
		t.Fatalf("failed to get user: %v", err)
	}

	loginRec := httptest.NewRecorder()
	loginReq, _ := http.NewRequest(http.MethodGet, "/login", http.NoBody)
	_, _ = printToken(loginRec, loginReq, &data{settings: &settings.Settings{Key: key}}, user, time.Hour)
	token := loginRec.Body.String()

	// Verify token works
	req, _ := http.NewRequest(http.MethodGet, "/", http.NoBody)
	req.Header.Set("X-Auth", token)
	rec := httptest.NewRecorder()
	handle(protected, "", st, server).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("token expected 200, got %d", rec.Code)
	}

	// Change password / update user (simulate time delay so update timestamp > token issued timestamp)
	time.Sleep(1100 * time.Millisecond)
	user.Password = "newpassword"
	if err := st.Users.Update(user, "Password"); err != nil {
		t.Fatalf("failed to update user: %v", err)
	}

	// Previously issued token must now be rejected with 401
	recUpdated := httptest.NewRecorder()
	reqUpdated, _ := http.NewRequest(http.MethodGet, "/", http.NoBody)
	reqUpdated.Header.Set("X-Auth", token)
	handle(protected, "", st, server).ServeHTTP(recUpdated, reqUpdated)
	if recUpdated.Code != http.StatusUnauthorized {
		t.Fatalf("token after password change expected 401, got %d", recUpdated.Code)
	}
}
