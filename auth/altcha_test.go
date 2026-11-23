package auth

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	altcha "github.com/altcha-org/altcha-lib-go/v2"
)

func testAltcha(t *testing.T) *Altcha {
	t.Helper()
	secret, err := NewAltchaSecret()
	if err != nil {
		t.Fatalf("NewAltchaSecret: %v", err)
	}
	return &Altcha{Secret: secret, Complexity: MinAltchaComplexity}
}

// solve plays the browser widget: it solves a challenge and encodes the
// payload the way the widget submits it.
func solve(t *testing.T, challenge altcha.Challenge) string {
	t.Helper()
	solution, err := altcha.SolveChallenge(altcha.SolveChallengeOptions{
		Challenge: challenge,
		DeriveKey: altcha.DeriveKeySHA(),
	})
	if err != nil || solution == nil {
		t.Fatalf("SolveChallenge: solution=%v err=%v", solution, err)
	}
	raw, err := json.Marshal(altcha.Payload{Challenge: challenge, Solution: *solution})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return base64.StdEncoding.EncodeToString(raw)
}

func newSolvedPayload(t *testing.T, a *Altcha) string {
	t.Helper()
	challenge, err := a.CreateChallenge()
	if err != nil {
		t.Fatalf("CreateChallenge: %v", err)
	}
	return solve(t, challenge)
}

func TestAltchaChallengeShape(t *testing.T) {
	a := testAltcha(t)
	challenge, err := a.CreateChallenge()
	if err != nil {
		t.Fatalf("CreateChallenge: %v", err)
	}
	p := challenge.Parameters
	if p.Algorithm != altchaAlgorithm || p.ExpiresAt == 0 || p.KeySignature == "" || challenge.Signature == "" {
		t.Fatalf("unexpected challenge parameters: %+v signature=%q", p, challenge.Signature)
	}
	raw, err := json.Marshal(challenge)
	if err != nil {
		t.Fatalf("marshal challenge: %v", err)
	}
	if strings.Contains(string(raw), a.Secret) {
		t.Fatal("challenge leaks the signing secret")
	}
}

func TestAltchaVerifyAcceptsSolvedChallengeOnce(t *testing.T) {
	a := testAltcha(t)
	payload := newSolvedPayload(t, a)

	if err := a.Verify(payload); err != nil {
		t.Fatalf("first use: expected success, got %v", err)
	}
	if err := a.Verify(payload); !errors.Is(err, ErrCaptchaRequired) {
		t.Fatalf("replay: expected ErrCaptchaRequired, got %v", err)
	}
}

func TestAltchaVerifyRejectsInvalidPayloads(t *testing.T) {
	a := testAltcha(t)

	other := testAltcha(t)
	foreign := newSolvedPayload(t, other)

	challenge, err := a.CreateChallenge()
	if err != nil {
		t.Fatalf("CreateChallenge: %v", err)
	}
	tampered := challenge
	tampered.Parameters.KeyPrefix = "00"
	tamperedPayload := solve(t, tampered)

	cases := map[string]string{
		"empty":           "",
		"not base64":      "%%%",
		"not json":        base64.StdEncoding.EncodeToString([]byte("nope")),
		"foreign secret":  foreign,
		"tampered params": tamperedPayload,
	}
	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			if err := a.Verify(payload); !errors.Is(err, ErrCaptchaRequired) {
				t.Fatalf("expected ErrCaptchaRequired, got %v", err)
			}
		})
	}
}

func TestJSONAuthCaptchaDisabledByDefault(t *testing.T) {
	if err := (JSONAuth{}).VerifyCaptcha(""); err != nil {
		t.Fatalf("captcha must be optional when ALTCHA is not configured, got %v", err)
	}
	if !(JSONAuth{ReCaptcha: &LegacyReCaptcha{Key: "k", Secret: "s"}}).LegacyReCaptchaConfigured() {
		t.Fatal("legacy reCAPTCHA settings must be detected")
	}
}
