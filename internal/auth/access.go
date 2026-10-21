package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"net/http"
	"strings"
	"time"
)

const (
	AccessCookie   = "hz_access"
	accessLifetime = 30 * 24 * time.Hour
)

// Access guards the shop with the shared server password. The cookie is a
// signed expiry; the signing key mixes in the password so changing it in
// .env logs every player out.
type Access struct {
	password []byte
	key      []byte
}

func NewAccess(password string, secret []byte) *Access {
	pw := sha256.Sum256([]byte(password))
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte("hamazon-access:"))
	mac.Write(pw[:])
	return &Access{password: []byte(password), key: mac.Sum(nil)}
}

func (a *Access) CheckPassword(candidate string) bool {
	want := sha256.Sum256(a.password)
	got := sha256.Sum256([]byte(candidate))
	return subtle.ConstantTimeCompare(want[:], got[:]) == 1
}

func (a *Access) Grant(w http.ResponseWriter, secure bool) {
	expires := time.Now().Add(accessLifetime)
	var exp [8]byte
	binary.BigEndian.PutUint64(exp[:], uint64(expires.Unix()))
	value := encode(exp[:]) + "." + encode(a.sign(exp[:]))
	http.SetCookie(w, &http.Cookie{
		Name: AccessCookie, Value: value, Path: "/", Expires: expires,
		HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode,
	})
}

func (a *Access) Valid(r *http.Request) bool {
	c, err := r.Cookie(AccessCookie)
	if err != nil {
		return false
	}
	expPart, sigPart, ok := strings.Cut(c.Value, ".")
	if !ok {
		return false
	}
	exp, err1 := decode(expPart)
	sig, err2 := decode(sigPart)
	if err1 != nil || err2 != nil || len(exp) != 8 {
		return false
	}
	if !hmac.Equal(sig, a.sign(exp)) {
		return false
	}
	return time.Now().Unix() < int64(binary.BigEndian.Uint64(exp))
}

func (a *Access) sign(data []byte) []byte {
	mac := hmac.New(sha256.New, a.key)
	mac.Write(data)
	return mac.Sum(nil)
}

func encode(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func decode(s string) ([]byte, error) { return base64.RawURLEncoding.DecodeString(s) }
