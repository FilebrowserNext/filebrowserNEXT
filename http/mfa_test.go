package fbhttp

import (
	"testing"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

func TestMFASealRoundTrip(t *testing.T) {
	key := []byte("test key")
	want := mfaTransaction{Purpose: "login", UserID: 42, ExpiresAt: time.Now().UTC().Truncate(time.Second)}
	sealed, err := sealMFA(key, want)
	if err != nil {
		t.Fatalf("seal MFA data: %v", err)
	}

	var got mfaTransaction
	if err := openMFA(key, sealed, &got); err != nil {
		t.Fatalf("open MFA data: %v", err)
	}
	if got.Purpose != want.Purpose || got.UserID != want.UserID || !got.ExpiresAt.Equal(want.ExpiresAt) {
		t.Fatalf("unexpected round trip value: %#v", got)
	}
	if err := openMFA([]byte("other key"), sealed, &got); err == nil {
		t.Fatal("MFA data opened with an unrelated key")
	}
}

func TestValidateTOTPUsesStoredAlgorithm(t *testing.T) {
	key, err := totp.Generate(totp.GenerateOpts{Issuer: "File Browser Next", AccountName: "test", SecretSize: 20, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1})
	if err != nil {
		t.Fatalf("generate TOTP key: %v", err)
	}
	code, err := totp.GenerateCodeCustom(key.Secret(), time.Now(), totp.ValidateOpts{Period: 30, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1})
	if err != nil {
		t.Fatalf("generate TOTP code: %v", err)
	}
	if !validateTOTP(code, key.Secret(), otp.AlgorithmSHA1.String()) {
		t.Fatal("valid SHA-1 TOTP code was rejected")
	}
	if validateTOTP(code, key.Secret(), otp.AlgorithmSHA256.String()) {
		t.Fatal("SHA-1 code was accepted as SHA-256")
	}
}
