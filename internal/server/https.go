package server

import (
	"net/http"
	"strings"
)

func (s *Server) isHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	if !s.fromProxy(r) {
		return false
	}
	hops := strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")
	return strings.EqualFold(strings.TrimSpace(hops[len(hops)-1]), "https")
}

func (s *Server) requireHTTPS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.SecureCookies && r.URL.Path != "/api/health" && !s.isHTTPS(r) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte("tuck only answers over https. put it behind an https reverse proxy and list that proxy in " +
				"TUCK_TRUSTED_PROXIES, or set TUCK_SECURE_COOKIES=false for local testing.\n"))
			return
		}
		if s.isHTTPS(r) {
			w.Header().Set("Strict-Transport-Security", "max-age=63072000")
		}
		next.ServeHTTP(w, r)
	})
}
