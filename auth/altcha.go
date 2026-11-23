package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	altcha "github.com/altcha-org/altcha-lib-go/v2"
	"github.com/jellydator/ttlcache/v3"
)

const (
	// DefaultAltchaComplexity is the default upper bound of the proof-of-work
	// counter. A browser needs well under a second to reach it.
	DefaultAltchaComplexity = 100000
	// MinAltchaComplexity keeps the proof of work meaningful.
	MinAltchaComplexity = 1000

	altchaAlgorithm   = "SHA-256"
	altchaKeyLength   = 32
	altchaTTL         = 10 * time.Minute
	altchaSecretBytes = 32
	// altchaMaxPayload bounds the base64 payload accepted from clients.
	altchaMaxPayload = 8 << 10
	// altchaMaxUsed bounds the replay cache so it cannot grow without limit.
	altchaMaxUsed = 100000
)

// ErrCaptchaRequired is returned when a captcha-protected action is attempted
// without a valid solution.
var ErrCaptchaRequired = errors.New("captcha verification failed")

// Altcha configures the self-hosted ALTCHA proof-of-work captcha. Challenges
// are created and verified locally: no third-party service is contacted.
type Altcha struct {
	// Secret signs challenges. It is generated automatically and never sent
	// to clients.
	Secret string `json:"secret" yaml:"secret"`
	// Complexity is the upper bound of the proof-of-work counter.
	Complexity int `json:"complexity" yaml:"complexity"`
}

// usedAltcha remembers solved challenges until they expire, so a solution
// can only be used once.
var (
	usedAltcha = ttlcache.New[string, struct{}](
		ttlcache.WithCapacity[string, struct{}](altchaMaxUsed),
		ttlcache.WithDisableTouchOnHit[string, struct{}](),
	)
	usedAltchaOnce sync.Once
)

// NewAltchaSecret returns a random hex-encoded secret for signing challenges.
func NewAltchaSecret() (string, error) {
	buf := make([]byte, altchaSecretBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate altcha secret: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// Enabled reports whether the captcha is configured.
func (a *Altcha) Enabled() bool {
	return a != nil && a.Secret != ""
}

func (a *Altcha) complexity() int {
	if a.Complexity < MinAltchaComplexity {
		return DefaultAltchaComplexity
	}
	return a.Complexity
}

// subkey derives a purpose-bound key from the stored secret so the challenge
// signature and the derived-key signature never share a key.
func (a *Altcha) subkey(purpose string) string {
	mac := hmac.New(sha256.New, []byte(a.Secret))
	mac.Write([]byte(purpose))
	return hex.EncodeToString(mac.Sum(nil))
}

// randomCounter picks the secret counter in [complexity/2, complexity] so the
// solving time stays predictable.
func (a *Altcha) randomCounter() (int, error) {
	high := a.complexity()
	low := high / 2
	n, err := rand.Int(rand.Reader, big.NewInt(int64(high-low+1)))
	if err != nil {
		return 0, fmt.Errorf("generate altcha counter: %w", err)
	}
	return low + int(n.Int64()), nil
}

// CreateChallenge returns a signed challenge for the browser widget.
func (a *Altcha) CreateChallenge() (altcha.Challenge, error) {
	if !a.Enabled() {
		return altcha.Challenge{}, errors.New("altcha is not configured")
	}
	counter, err := a.randomCounter()
	if err != nil {
		return altcha.Challenge{}, err
	}
	expiresAt := time.Now().Add(altchaTTL)
	return altcha.CreateChallenge(altcha.CreateChallengeOptions{
		Algorithm:              altchaAlgorithm,
		Cost:                   1,
		Counter:                &counter,
		DeriveKey:              altcha.DeriveKeySHA(),
		ExpiresAt:              &expiresAt,
		HMACSignatureSecret:    a.subkey("challenge"),
		HMACKeySignatureSecret: a.subkey("key"),
		KeyLength:              altchaKeyLength,
	})
}

// Verify checks a base64 payload produced by the widget. A payload is
// accepted at most once.
func (a *Altcha) Verify(encoded string) error {
	if !a.Enabled() {
		return nil
	}
	payload, err := decodeAltchaPayload(encoded)
	if err != nil {
		return ErrCaptchaRequired
	}
	if payload.Challenge.Parameters.Algorithm != altchaAlgorithm {
		return ErrCaptchaRequired
	}

	result, err := altcha.VerifySolution(altcha.VerifySolutionOptions{
		Challenge:              payload.Challenge,
		Solution:               payload.Solution,
		DeriveKey:              altcha.DeriveKeySHA(),
		HMACSignatureSecret:    a.subkey("challenge"),
		HMACKeySignatureSecret: a.subkey("key"),
	})
	if err != nil {
		return fmt.Errorf("verify altcha solution: %w", err)
	}
	if !result.Verified {
		return ErrCaptchaRequired
	}
	return markAltchaUsed(payload.Challenge.Parameters)
}

func decodeAltchaPayload(encoded string) (altcha.Payload, error) {
	var payload altcha.Payload
	if encoded == "" || len(encoded) > altchaMaxPayload {
		return payload, ErrCaptchaRequired
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return payload, err
	}
	err = json.Unmarshal(raw, &payload)
	return payload, err
}

// markAltchaUsed records the challenge nonce. Signed challenges always carry
// an expiry, which bounds how long the nonce must be remembered.
func markAltchaUsed(params altcha.ChallengeParameters) error {
	usedAltchaOnce.Do(func() { go usedAltcha.Start() })

	ttl := time.Until(time.Unix(params.ExpiresAt, 0))
	if params.ExpiresAt == 0 || ttl <= 0 {
		return ErrCaptchaRequired
	}
	_, used := usedAltcha.GetOrSet(params.Nonce, struct{}{}, ttlcache.WithTTL[string, struct{}](ttl))
	if used {
		return ErrCaptchaRequired
	}
	return nil
}
