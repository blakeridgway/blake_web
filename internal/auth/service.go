package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"strings"
	"time"
)

type Service struct {
	username string
	hash     string // SHA-256 hex of password
	secret   []byte
}

func New(username, hash, secret string) *Service {
	return &Service{
		username: username,
		hash:     strings.ToUpper(hash),
		secret:   []byte(secret),
	}
}

func (s *Service) Authenticate(username, password string) bool {
	if !strings.EqualFold(username, s.username) {
		return false
	}
	h := sha256.Sum256([]byte(password))
	return strings.EqualFold(hex.EncodeToString(h[:]), s.hash)
}

func (s *Service) SetCookie(w http.ResponseWriter, username string) {
	value := s.sign(username)
	http.SetCookie(w, &http.Cookie{
		Name:     "admin_session",
		Value:    value,
		Path:     "/admin",
		HttpOnly: true,
		Secure:   false, // set true behind TLS in prod
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(24 * time.Hour),
	})
}

func (s *Service) ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:    "admin_session",
		Value:   "",
		Path:    "/admin",
		MaxAge:  -1,
		Expires: time.Unix(0, 0),
	})
}

func (s *Service) IsAuthenticated(r *http.Request) bool {
	c, err := r.Cookie("admin_session")
	if err != nil || c.Value == "" {
		return false
	}
	_, ok := s.verify(c.Value)
	return ok
}

func (s *Service) sign(username string) string {
	b := base64.StdEncoding.EncodeToString([]byte(username))
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(b))
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return b + "." + sig
}

func (s *Service) verify(value string) (string, bool) {
	parts := strings.SplitN(value, ".", 2)
	if len(parts) != 2 {
		return "", false
	}
	b, sig := parts[0], parts[1]
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(b))
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return "", false
	}
	name, err := base64.StdEncoding.DecodeString(b)
	if err != nil {
		return "", false
	}
	return string(name), true
}
