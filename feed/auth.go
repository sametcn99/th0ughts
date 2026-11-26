package feed

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"github.com/sametcn99/th0ughts/storage"
	"golang.org/x/crypto/argon2"
)

type Vault struct{ aead cipher.AEAD }

func NewVault(key string) (Vault, error) {
	if len(key) < 32 {
		return Vault{}, errors.New("master key must contain at least 32 bytes")
	}
	sum := sha256.Sum256([]byte(key))
	b, e := aes.NewCipher(sum[:])
	if e != nil {
		return Vault{}, e
	}
	a, e := cipher.NewGCM(b)
	return Vault{a}, e
}
func (v Vault) Seal(label string, b []byte) []byte {
	n := make([]byte, v.aead.NonceSize())
	rand.Read(n)
	return v.aead.Seal(n, n, b, []byte(label))
}
func (v Vault) Open(label string, b []byte) ([]byte, error) {
	n := v.aead.NonceSize()
	if len(b) < n {
		return nil, Invalid
	}
	return v.aead.Open(nil, b[:n], b[n:], []byte(label))
}

type Account struct {
	ID, Salt, Hash string
	TOTP           []byte
	LastStep       int64
	Recovery       []string
	Credentials    []webauthn.Credential
}

func (a Account) WebAuthnID() []byte                         { return []byte(a.ID) }
func (a Account) WebAuthnName() string                       { return "owner" }
func (a Account) WebAuthnDisplayName() string                { return "Owner" }
func (a Account) WebAuthnCredentials() []webauthn.Credential { return a.Credentials }

type Access struct {
	Name    string    `json:"name"`
	Scopes  []string  `json:"scopes"`
	Expires time.Time `json:"expires"`
	Issued  time.Time `json:"issued"`
}
type Auth struct {
	DB    storage.Store
	Vault Vault
	mu    sync.Mutex
	slots chan struct{}
}

func NewAuth(db storage.Store, v Vault) *Auth {
	return &Auth{DB: db, Vault: v, slots: make(chan struct{}, 2)}
}
func Digest(raw string) string { sum := sha256.Sum256([]byte(raw)); return hex.EncodeToString(sum[:]) }
func (a *Auth) Account(ctx context.Context) (Account, error) {
	return Read[Account](ctx, a.DB, "auth", "owner")
}
func password(p, salt string) string {
	return base64.RawStdEncoding.EncodeToString(argon2.IDKey([]byte(p), []byte(salt), 3, 64*1024, 2, 32))
}
func (a *Auth) Setup(ctx context.Context, p string, reset bool) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(p) < 12 || len(p) > 256 {
		return Invalid
	}
	_, e := a.Account(ctx)
	if !reset && e == nil {
		return Conflict
	}
	if e != nil && !errors.Is(e, storage.ErrMissing) {
		return e
	}
	salt := ID(16)
	u := Account{ID: ID(16), Salt: salt, Hash: password(p, salt)}
	return Write(ctx, a.DB, "auth", "owner", u, 0, false)
}
func (a *Auth) Check(ctx context.Context, p, code string) error {
	if len(p) > 256 {
		return Denied
	}
	select {
	case a.slots <- struct{}{}:
		defer func() { <-a.slots }()
	default:
		return Denied
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	u, e := a.Account(ctx)
	if e != nil {
		return Denied
	}
	if subtle.ConstantTimeCompare([]byte(u.Hash), []byte(password(p, u.Salt))) != 1 {
		return Denied
	}
	if len(u.TOTP) == 0 {
		return nil
	}
	secret, e := a.Vault.Open("totp", u.TOTP)
	if e != nil {
		return e
	}
	step := time.Now().Unix() / 30
	matched := int64(0)
	for delta := int64(-1); delta <= 1; delta++ {
		candidate := step + delta
		ok, _ := totp.ValidateCustom(code, string(secret), time.Unix(candidate*30, 0), totp.ValidateOpts{Period: 30, Skew: 0, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1})
		if ok {
			matched = candidate
			break
		}
	}
	if matched > u.LastStep {
		u.LastStep = matched
	} else {
		ok := false
		for i, h := range u.Recovery {
			if subtle.ConstantTimeCompare([]byte(h), []byte(Digest(code))) == 1 {
				u.Recovery = append(u.Recovery[:i], u.Recovery[i+1:]...)
				ok = true
				break
			}
		}
		if !ok {
			return Denied
		}
	}
	return Write(ctx, a.DB, "auth", "owner", u, 0, false)
}
func (a *Auth) EnableOTP(ctx context.Context, actor Actor, secret, code string) ([]string, error) {
	if !actor.Recent() || len(secret) < 16 || !totp.Validate(code, secret) {
		return nil, Denied
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	u, e := a.Account(ctx)
	if e != nil {
		return nil, e
	}
	u.TOTP = a.Vault.Seal("totp", []byte(secret))
	u.Recovery = nil
	codes := []string{}
	for i := 0; i < 8; i++ {
		x := ID(10)
		codes = append(codes, x)
		u.Recovery = append(u.Recovery, Digest(x))
	}
	return codes, Write(ctx, a.DB, "auth", "owner", u, 0, false)
}
func (a *Auth) Credential(ctx context.Context, cred webauthn.Credential, expectedID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	u, e := a.Account(ctx)
	if e != nil {
		return e
	}
	if u.ID != expectedID {
		return Denied
	}
	found := false
	for i, c := range u.Credentials {
		if string(c.ID) == string(cred.ID) {
			u.Credentials[i] = cred
			found = true
			break
		}
	}
	if !found {
		u.Credentials = append(u.Credentials, cred)
	}
	return Write(ctx, a.DB, "auth", "owner", u, 0, false)
}

type grant struct {
	Access
	Account string `json:"account"`
}

func (a *Auth) issue(ctx context.Context, kind string, t Access) (string, error) {
	u, e := a.Account(ctx)
	if e != nil {
		return "", e
	}
	raw := ID(32)
	t.Issued = time.Now().UTC()
	return raw, Write(ctx, a.DB, kind, Digest(raw), grant{t, u.ID}, t.Issued.Unix(), false)
}
func (a *Auth) Session(ctx context.Context) (string, error) {
	return a.issue(ctx, "session", Access{Expires: time.Now().Add(12 * time.Hour)})
}
func (a *Auth) Token(ctx context.Context, user Actor, t Access) (string, error) {
	if !user.Recent() {
		return "", Denied
	}
	if len(t.Name) == 0 || len(t.Name) > 100 || len(t.Scopes) == 0 || !t.Expires.After(time.Now()) || t.Expires.After(time.Now().AddDate(1, 0, 1)) {
		return "", Invalid
	}
	for _, s := range t.Scopes {
		if !strings.Contains("|posts:read|posts:write|settings|themes|media|", "|"+s+"|") || strings.Contains(s, "|") {
			return "", Invalid
		}
	}
	return a.issue(ctx, "token", t)
}
func (a *Auth) Resolve(ctx context.Context, kind, raw string) Actor {
	if len(raw) != 64 {
		return Actor{}
	}
	t, e := Read[grant](ctx, a.DB, kind, Digest(raw))
	if e != nil || time.Now().After(t.Expires) {
		return Actor{}
	}
	u, e := a.Account(ctx)
	if e != nil || u.ID != t.Account {
		return Actor{}
	}
	return Actor{Owner: kind == "session", Scopes: t.Scopes, Fresh: t.Issued, Session: Digest(raw)}
}
