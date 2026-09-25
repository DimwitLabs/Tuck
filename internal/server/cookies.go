package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	sessionCookie     = "tuck_session"
	gateCookie        = "tuck_gate"
	deviceCookie      = "tuck_device"
	registerCookie    = "tuck_pk_add"
	gatePasskeyCookie = "tuck_pk_gate"
	deviceTTL         = 90 * 24 * time.Hour
)

func (s *Server) cookieName(base string) string {
	if s.cfg.SecureCookies {
		return "__Host-" + base
	}
	return base
}

func (s *Server) setCookie(w http.ResponseWriter, base, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     s.cookieName(base),
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   s.cfg.SecureCookies,
		SameSite: http.SameSiteStrictMode,
	})
}

func (s *Server) cookie(r *http.Request, base string) string {
	c, err := r.Cookie(s.cookieName(base))
	if err != nil {
		return ""
	}
	return c.Value
}

func (s *Server) signed(purpose string, ttl time.Duration) string {
	expires := strconv.FormatInt(time.Now().Add(ttl).Unix(), 10)
	return expires + "." + hex.EncodeToString(s.mac(purpose, expires))
}

func (s *Server) verify(purpose, value string) bool {
	expires, sig, ok := strings.Cut(value, ".")
	if !ok {
		return false
	}
	unix, err := strconv.ParseInt(expires, 10, 64)
	if err != nil || time.Now().Unix() > unix {
		return false
	}
	given, err := hex.DecodeString(sig)
	return err == nil && hmac.Equal(given, s.mac(purpose, expires))
}

// Carries a WebAuthn challenge back to us untampered, so a ceremony needs no server-side state.
func (s *Server) sealed(purpose string, payload []byte, ttl time.Duration) string {
	body := base64.RawURLEncoding.EncodeToString(payload)
	expires := strconv.FormatInt(time.Now().Add(ttl).Unix(), 10)
	return expires + "." + body + "." + hex.EncodeToString(s.mac(purpose+"|"+body, expires))
}

func (s *Server) unsealed(purpose, value string) ([]byte, bool) {
	expires, rest, ok := strings.Cut(value, ".")
	if !ok {
		return nil, false
	}
	body, sig, ok := strings.Cut(rest, ".")
	if !ok {
		return nil, false
	}
	unix, err := strconv.ParseInt(expires, 10, 64)
	if err != nil || time.Now().Unix() > unix {
		return nil, false
	}
	given, err := hex.DecodeString(sig)
	if err != nil || !hmac.Equal(given, s.mac(purpose+"|"+body, expires)) {
		return nil, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(body)
	return payload, err == nil
}

func (s *Server) mac(purpose, expires string) []byte {
	key := hmac.New(sha256.New, s.cfg.Secret)
	key.Write([]byte(purpose))
	mac := hmac.New(sha256.New, key.Sum(nil))
	mac.Write([]byte(expires))
	return mac.Sum(nil)
}

func (s *Server) markDevice(w http.ResponseWriter, userID string) {
	s.setCookie(w, deviceCookie, s.signed("device:"+userID, deviceTTL), int(deviceTTL.Seconds()))
}

func (s *Server) knownDevice(r *http.Request, userID string) bool {
	return s.verify("device:"+userID, s.cookie(r, deviceCookie))
}
