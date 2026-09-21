package server

import (
	"crypto/subtle"
	"net/http"
	"time"
)

const gateTTL = 15 * time.Minute

var errGateClosed = fail(http.StatusForbidden, "closed")

func (s *Server) gatePurpose() string { return "gate:" + s.cfg.GatePassword }

func (s *Server) gatePassed(r *http.Request) bool {
	if s.cfg.GatePassword == "" {
		return true
	}
	if _, err := s.session(r); err == nil {
		return true
	}
	return s.verify(s.gatePurpose(), s.cookie(r, gateCookie))
}

func (s *Server) gate(w http.ResponseWriter, r *http.Request) error {
	var body struct {
		Typed string `json:"typed"`
	}
	if err := readJSON(r, &body); err != nil {
		return err
	}
	if s.cfg.GatePassword == "" {
		writeJSON(w, http.StatusOK, map[string]bool{"open": true})
		return nil
	}
	password := []byte(s.cfg.GatePassword)
	typed := []byte(body.Typed)
	if !s.gateByIP.allow(limiterKey(s.clientIP(r))) || len(typed) < len(password) ||
		subtle.ConstantTimeCompare(typed[len(typed)-len(password):], password) != 1 {
		writeJSON(w, http.StatusOK, map[string]bool{"open": false})
		return nil
	}
	s.setCookie(w, gateCookie, s.signed(s.gatePurpose(), gateTTL), int(gateTTL.Seconds()))
	s.audit(r, "gate_opened")
	writeJSON(w, http.StatusOK, map[string]bool{"open": true})
	return nil
}
