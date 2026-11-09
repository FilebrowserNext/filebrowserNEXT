package fbhttp

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	webauthn "github.com/go-webauthn/webauthn/webauthn"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"

	fberrors "github.com/filebrowser/filebrowser/v2/errors"
	"github.com/filebrowser/filebrowser/v2/settings"
	"github.com/filebrowser/filebrowser/v2/users"
)

const mfaTransactionLifetime = 5 * time.Minute

type mfaRecord struct {
	TOTPSecret    string                `json:"totpSecret,omitempty"`
	TOTPAlgorithm string                `json:"totpAlgorithm,omitempty"`
	UserHandle    []byte                `json:"userHandle,omitempty"`
	Credentials   []webauthn.Credential `json:"credentials,omitempty"`
}

type mfaTransaction struct {
	Purpose       string               `json:"purpose"`
	UserID        uint                 `json:"userID"`
	ExpiresAt     time.Time            `json:"expiresAt"`
	TOTPSecret    string               `json:"totpSecret,omitempty"`
	TOTPAlgorithm string               `json:"totpAlgorithm,omitempty"`
	Session       webauthn.SessionData `json:"session,omitempty"`
}

type passkeyUser struct {
	user   *users.User
	record *mfaRecord
}

func (u passkeyUser) WebAuthnID() []byte                         { return u.record.UserHandle }
func (u passkeyUser) WebAuthnName() string                       { return u.user.Username }
func (u passkeyUser) WebAuthnDisplayName() string                { return u.user.Username }
func (u passkeyUser) WebAuthnCredentials() []webauthn.Credential { return u.record.Credentials }

func mfaKey(key []byte) []byte {
	sum := sha256.Sum256(append([]byte("filebrowser-next/mfa/v1"), key...))
	return sum[:]
}

func sealMFA(key []byte, value any) (string, error) {
	plain, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(mfaKey(key))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(gcm.Seal(nonce, nonce, plain, nil)), nil
}

func openMFA(key []byte, sealed string, value any) error {
	raw, err := base64.RawURLEncoding.DecodeString(sealed)
	if err != nil {
		return err
	}
	block, err := aes.NewCipher(mfaKey(key))
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	if len(raw) < gcm.NonceSize() {
		return errors.New("invalid MFA data")
	}
	plain, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
	if err != nil {
		return err
	}
	return json.Unmarshal(plain, value)
}

func loadMFA(d *data, userID uint) (*mfaRecord, error) {
	if d.store.MFA == nil {
		return nil, errors.New("MFA storage is unavailable")
	}
	ciphertext, err := d.store.MFA.Get(userID)
	if errors.Is(err, fberrors.ErrNotExist) {
		return &mfaRecord{}, nil
	}
	if err != nil {
		return nil, err
	}
	var record mfaRecord
	if err := openMFA(d.settings.Key, string(ciphertext), &record); err != nil {
		return nil, err
	}
	return &record, nil
}

func saveMFA(d *data, userID uint, record *mfaRecord) error {
	if record.TOTPSecret == "" && len(record.Credentials) == 0 && len(record.UserHandle) == 0 {
		return d.store.MFA.Delete(userID)
	}
	ciphertext, err := sealMFA(d.settings.Key, record)
	if err != nil {
		return err
	}
	return d.store.MFA.Save(userID, []byte(ciphertext))
}

func passkeyTransactionFromBody(r *http.Request) (string, error) {
	if r.Body == nil {
		return "", errors.New("missing request body")
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxAuthBodySize+1))
	if err != nil || len(body) > maxAuthBodySize {
		return "", errors.New("invalid request body")
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	var request struct {
		Transaction string `json:"transaction"`
	}
	if err := json.Unmarshal(body, &request); err != nil || request.Transaction == "" {
		return "", errors.New("invalid transaction")
	}
	return request.Transaction, nil
}

func issueTransaction(d *data, transaction mfaTransaction) (string, error) {
	transaction.ExpiresAt = time.Now().Add(mfaTransactionLifetime)
	return sealMFA(d.settings.Key, transaction)
}

func readTransaction(d *data, token, purpose string) (*mfaTransaction, error) {
	var transaction mfaTransaction
	if err := openMFA(d.settings.Key, token, &transaction); err != nil {
		return nil, err
	}
	if transaction.Purpose != purpose || time.Now().After(transaction.ExpiresAt) {
		return nil, errors.New("expired MFA transaction")
	}
	return &transaction, nil
}

func passkeyConfig(server *settings.Server) (*webauthn.WebAuthn, error) {
	if server.WebAuthnRPID == "" || len(server.WebAuthnOrigins) == 0 {
		return nil, errors.New("passkeys are not configured")
	}
	return webauthn.New(&webauthn.Config{RPID: server.WebAuthnRPID, RPDisplayName: "File Browser Next", RPOrigins: server.WebAuthnOrigins})
}

func requireCurrentPassword(r *http.Request, d *data) error {
	if d.settings.AuthMethod != "json" {
		return nil
	}
	var body struct {
		CurrentPassword string `json:"current_password"`
	}
	if r.Body == nil || json.NewDecoder(r.Body).Decode(&body) != nil || !users.CheckPwd(body.CurrentPassword, d.user.Password) {
		return fberrors.ErrCurrentPasswordIncorrect
	}
	return nil
}

func mfaStatusHandler(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	record, err := loadMFA(d, d.user.ID)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return renderJSON(w, r, map[string]any{"totp": record.TOTPSecret != "", "passkeys": len(record.Credentials)})
}

func totpBeginHandler(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	if err := requireCurrentPassword(r, d); err != nil {
		return http.StatusBadRequest, err
	}
	key, err := totp.Generate(totp.GenerateOpts{Issuer: "File Browser Next", AccountName: d.user.Username, SecretSize: 20, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1})
	if err != nil {
		return http.StatusInternalServerError, err
	}
	transaction, err := issueTransaction(d, mfaTransaction{Purpose: "totp-enroll", UserID: d.user.ID, TOTPSecret: key.Secret(), TOTPAlgorithm: otp.AlgorithmSHA1.String()})
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return renderJSON(w, r, map[string]string{"transaction": transaction, "uri": key.URL()})
}

func totpConfirmHandler(_ http.ResponseWriter, r *http.Request, d *data) (int, error) {
	var body struct {
		Transaction string `json:"transaction"`
		Code        string `json:"code"`
	}
	if r.Body == nil || json.NewDecoder(r.Body).Decode(&body) != nil {
		return http.StatusBadRequest, nil
	}
	transaction, err := readTransaction(d, body.Transaction, "totp-enroll")
	if err != nil || transaction.UserID != d.user.ID || !validateTOTP(body.Code, transaction.TOTPSecret, transaction.TOTPAlgorithm) {
		return http.StatusBadRequest, nil
	}
	record, err := loadMFA(d, d.user.ID)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	record.TOTPSecret = transaction.TOTPSecret
	record.TOTPAlgorithm = transaction.TOTPAlgorithm
	if err := saveMFA(d, d.user.ID, record); err != nil {
		return http.StatusInternalServerError, err
	}
	return http.StatusNoContent, nil
}

func validateTOTP(code, secret, algorithm string) bool {
	// Existing pre-release enrollments were created with SHA-256. New
	// enrollments use the TOTP default (SHA-1), which has the widest support
	// among authenticator applications.
	selected := otp.AlgorithmSHA1
	if algorithm == otp.AlgorithmSHA256.String() || algorithm == "" {
		selected = otp.AlgorithmSHA256
	}
	valid, err := totp.ValidateCustom(strings.TrimSpace(code), secret, time.Now(), totp.ValidateOpts{Period: 30, Skew: 1, Digits: otp.DigitsSix, Algorithm: selected})
	return err == nil && valid
}

func completeTOTPLogin(w http.ResponseWriter, r *http.Request, d *data, tokenExpiration time.Duration) (int, error) {
	if !loginLimiter.allow(clientIP(r)) {
		return http.StatusTooManyRequests, nil
	}

	var body struct {
		Transaction string `json:"transaction"`
		Code        string `json:"code"`
	}
	if r.Body == nil || json.NewDecoder(r.Body).Decode(&body) != nil {
		return http.StatusBadRequest, nil
	}
	transaction, err := readTransaction(d, body.Transaction, "login")
	if err != nil {
		return http.StatusForbidden, nil
	}
	user, err := d.store.Users.Get(d.server.Root, d.server.FollowExternalSymlinks, transaction.UserID)
	if err != nil {
		return http.StatusForbidden, nil
	}
	record, err := loadMFA(d, user.ID)
	if err != nil || record.TOTPSecret == "" || !validateTOTP(body.Code, record.TOTPSecret, record.TOTPAlgorithm) {
		return http.StatusForbidden, nil
	}
	return printToken(w, r, d, user, tokenExpiration)
}

func passkeyBeginHandler(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	if err := requireCurrentPassword(r, d); err != nil {
		return http.StatusBadRequest, err
	}
	service, err := passkeyConfig(d.server)
	if err != nil {
		return http.StatusServiceUnavailable, err
	}
	record, err := loadMFA(d, d.user.ID)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	if len(record.UserHandle) == 0 {
		record.UserHandle = make([]byte, 32)
		if _, err = rand.Read(record.UserHandle); err != nil {
			return http.StatusInternalServerError, err
		}
	}
	options, session, err := service.BeginRegistration(passkeyUser{d.user, record}, webauthn.WithAuthenticatorSelection(protocol.AuthenticatorSelection{UserVerification: protocol.VerificationRequired}))
	if err != nil {
		return http.StatusInternalServerError, err
	}
	transaction, err := issueTransaction(d, mfaTransaction{Purpose: "passkey-enroll", UserID: d.user.ID, Session: *session})
	if err != nil {
		return http.StatusInternalServerError, err
	}
	if err := saveMFA(d, d.user.ID, record); err != nil {
		return http.StatusInternalServerError, err
	}
	return renderJSON(w, r, map[string]any{"transaction": transaction, "publicKey": options})
}

func passkeyFinishHandler(_ http.ResponseWriter, r *http.Request, d *data) (int, error) {
	token, err := passkeyTransactionFromBody(r)
	if err != nil {
		return http.StatusBadRequest, nil
	}
	transaction, err := readTransaction(d, token, "passkey-enroll")
	if err != nil || transaction.UserID != d.user.ID {
		return http.StatusBadRequest, nil
	}
	service, err := passkeyConfig(d.server)
	if err != nil {
		return http.StatusServiceUnavailable, err
	}
	record, err := loadMFA(d, d.user.ID)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	credential, err := service.FinishRegistration(passkeyUser{d.user, record}, transaction.Session, r)
	if err != nil {
		return http.StatusBadRequest, nil
	}
	record.Credentials = append(record.Credentials, *credential)
	if err := saveMFA(d, d.user.ID, record); err != nil {
		return http.StatusInternalServerError, err
	}
	return http.StatusNoContent, nil
}

func passkeyLoginBegin(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	if !loginLimiter.allow(clientIP(r)) {
		return http.StatusTooManyRequests, nil
	}

	var body struct {
		Transaction string `json:"transaction"`
	}
	if r.Body == nil || json.NewDecoder(r.Body).Decode(&body) != nil {
		return http.StatusBadRequest, nil
	}
	transaction, err := readTransaction(d, body.Transaction, "login")
	if err != nil {
		return http.StatusForbidden, nil
	}
	user, err := d.store.Users.Get(d.server.Root, d.server.FollowExternalSymlinks, transaction.UserID)
	if err != nil {
		return http.StatusForbidden, nil
	}
	record, err := loadMFA(d, user.ID)
	if err != nil || len(record.Credentials) == 0 {
		return http.StatusForbidden, nil
	}
	service, err := passkeyConfig(d.server)
	if err != nil {
		return http.StatusServiceUnavailable, err
	}
	options, session, err := service.BeginLogin(passkeyUser{user, record}, webauthn.WithUserVerification(protocol.VerificationRequired))
	if err != nil {
		return http.StatusInternalServerError, err
	}
	transaction.Session = *session
	next, err := issueTransaction(d, *transaction)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return renderJSON(w, r, map[string]any{"transaction": next, "publicKey": options})
}

func completePasskeyLogin(w http.ResponseWriter, r *http.Request, d *data, tokenExpiration time.Duration) (int, error) {
	if !loginLimiter.allow(clientIP(r)) {
		return http.StatusTooManyRequests, nil
	}

	token, err := passkeyTransactionFromBody(r)
	if err != nil {
		return http.StatusBadRequest, nil
	}
	transaction, err := readTransaction(d, token, "login")
	if err != nil {
		return http.StatusForbidden, nil
	}
	user, err := d.store.Users.Get(d.server.Root, d.server.FollowExternalSymlinks, transaction.UserID)
	if err != nil {
		return http.StatusForbidden, nil
	}
	record, err := loadMFA(d, user.ID)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	service, err := passkeyConfig(d.server)
	if err != nil {
		return http.StatusServiceUnavailable, err
	}
	credential, err := service.FinishLogin(passkeyUser{user, record}, transaction.Session, r)
	if err != nil {
		return http.StatusForbidden, nil
	}
	for i := range record.Credentials {
		if string(record.Credentials[i].ID) == string(credential.ID) {
			record.Credentials[i] = *credential
			break
		}
	}
	if err := saveMFA(d, user.ID, record); err != nil {
		return http.StatusInternalServerError, err
	}
	return printToken(w, r, d, user, tokenExpiration)
}

func mfaDeleteTOTPHandler(_ http.ResponseWriter, r *http.Request, d *data) (int, error) {
	if err := requireCurrentPassword(r, d); err != nil {
		return http.StatusBadRequest, err
	}
	record, err := loadMFA(d, d.user.ID)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	record.TOTPSecret = ""
	record.TOTPAlgorithm = ""
	if err := saveMFA(d, d.user.ID, record); err != nil {
		return http.StatusInternalServerError, err
	}
	return http.StatusNoContent, nil
}

func mfaDeletePasskeyHandler(_ http.ResponseWriter, r *http.Request, d *data) (int, error) {
	if err := requireCurrentPassword(r, d); err != nil {
		return http.StatusBadRequest, err
	}
	var body struct {
		ID string `json:"id"`
	}
	if r.Body == nil || json.NewDecoder(r.Body).Decode(&body) != nil {
		return http.StatusBadRequest, nil
	}
	id, err := base64.RawURLEncoding.DecodeString(body.ID)
	if err != nil {
		return http.StatusBadRequest, nil
	}
	record, err := loadMFA(d, d.user.ID)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	for i, credential := range record.Credentials {
		if string(credential.ID) == string(id) {
			record.Credentials = append(record.Credentials[:i], record.Credentials[i+1:]...)
			if err := saveMFA(d, d.user.ID, record); err != nil {
				return http.StatusInternalServerError, err
			}
			return http.StatusNoContent, nil
		}
	}
	return http.StatusNotFound, fmt.Errorf("passkey not found")
}
