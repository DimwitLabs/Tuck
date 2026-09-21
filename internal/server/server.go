package server

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"html"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"path"
	"strings"
	"time"

	"github.com/DimwitLabs/tuck/internal/store"
)

type Features string

const (
	FeaturesBoth  Features = "both"
	FeaturesSSH   Features = "ssh"
	FeaturesFiles Features = "files"
)

func (f Features) allows(kind string) bool {
	switch kind {
	case "manifest":
		return true
	case "file":
		return f != FeaturesSSH
	}
	return f != FeaturesFiles
}

type Config struct {
	Features       Features
	SecureCookies  bool
	TrustedProxies []netip.Prefix
	SessionTTL     time.Duration
	MaxFileBytes   int64
	Quota          store.Quota
	Lockout        store.Lockout
	Secret         []byte
	GatePassword   string
	GateWord       string
}

type Server struct {
	store       *store.Store
	cfg         Config
	static      fs.FS
	loginByIP   *limiter
	loginByUser *limiter
	signupByIP  *limiter
	answerByIP  *limiter
	gateByIP    *limiter
	rekeyTries  *limiter
	public      chan struct{}
}

func New(st *store.Store, cfg Config, static fs.FS) *Server {
	return &Server{
		store:       st,
		cfg:         cfg,
		static:      static,
		loginByIP:   newLimiter(20, 15*time.Minute),
		loginByUser: newLimiter(10, 15*time.Minute),
		signupByIP:  newLimiter(5, time.Hour),
		answerByIP:  newLimiter(60, 15*time.Minute),
		gateByIP:    newLimiter(300, 15*time.Minute),
		rekeyTries:  newLimiter(5, time.Hour),
		public:      make(chan struct{}, publicSlots),
	}
}

func (s *Server) Janitor(ctx context.Context) {
	t := time.NewTicker(10 * time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := s.store.PurgeExpiredSessions(ctx); err != nil {
				slog.Error("purging expired sessions", "err", err)
			}
			for _, l := range []*limiter{s.loginByIP, s.loginByUser, s.signupByIP, s.answerByIP, s.gateByIP, s.rekeyTries} {
				l.sweep()
			}
		}
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.bounded(s.wrap(s.health)))
	mux.HandleFunc("GET /api/status", s.bounded(s.wrap(s.status)))
	mux.HandleFunc("GET /api/prelogin", s.bounded(s.wrap(s.prelogin)))
	mux.HandleFunc("POST /api/gate", s.bounded(s.wrap(s.gate)))
	mux.HandleFunc("POST /api/signup", s.bounded(s.wrap(s.signup)))
	mux.HandleFunc("POST /api/login", s.bounded(s.wrap(s.login)))
	mux.HandleFunc("POST /api/logout", s.wrap(s.logout))
	mux.HandleFunc("POST /api/unlock/start", s.wrap(s.unlockStart))
	mux.HandleFunc("POST /api/unlock/answer", s.wrap(s.unlockAnswer))
	mux.HandleFunc("POST /api/lock", s.wrap(s.lock))
	mux.HandleFunc("PUT /api/account/keys", s.wrap(s.rekey))
	mux.HandleFunc("GET /api/items", s.wrap(s.listItems))
	mux.HandleFunc("PUT /api/items/{id}", s.wrap(s.putItem))
	mux.HandleFunc("DELETE /api/items/{id}", s.wrap(s.deleteItem))
	mux.HandleFunc("PUT /api/items/{id}/file", s.wrap(s.putFile))
	mux.HandleFunc("GET /api/items/{id}/file", s.wrap(s.getFile))
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no such endpoint"})
	})
	mux.Handle("/", s.spa())
	return securityHeaders(s.requireHTTPS(csrfGuard(mux)))
}

const publicSlots = 32

func (s *Server) bounded(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		select {
		case s.public <- struct{}{}:
			defer func() { <-s.public }()
			h(w, r)
		default:
			w.Header().Set("Retry-After", "5")
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "tuck is busy; try again in a moment"})
		}
	}
}

const contentSecurityPolicy = "default-src 'self'; script-src 'self' 'wasm-unsafe-eval'; style-src 'self'; " +
	"img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'"

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", contentSecurityPolicy)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			h.Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

func csrfGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") && r.Method != http.MethodGet && r.Method != http.MethodHead &&
			r.Header.Get("X-Tuck-Request") != "1" {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "missing X-Tuck-Request header"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) spa() http.Handler {
	files := http.FileServerFS(s.static)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name == "" {
			name = "index.html"
		}
		if info, err := fs.Stat(s.static, name); err != nil || info.IsDir() {
			name = "index.html"
			r = r.Clone(r.Context())
			r.URL.Path = "/"
		}
		if strings.HasPrefix(name, "assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		if name == "index.html" && s.cfg.GatePassword != "" {
			page, err := fs.ReadFile(s.static, name)
			if err != nil {
				http.Error(w, "missing page", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(bytes.Replace(page, []byte("<title>Tuck</title>"), []byte("<title>"+html.EscapeString(s.cfg.GateWord)+"</title>"), 1))
			return
		}
		files.ServeHTTP(w, r)
	})
}

func newToken() (string, []byte, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	return token, sha([]byte(token)), nil
}

func (s *Server) startSession(ctx context.Context, w http.ResponseWriter, userID string, stage int) error {
	token, hash, err := newToken()
	if err != nil {
		return err
	}
	if err := s.store.CreateSession(ctx, hash, userID, stage, s.cfg.SessionTTL); err != nil {
		return err
	}
	s.setSessionCookie(w, token, time.Now().Add(s.cfg.SessionTTL))
	return nil
}

func (s *Server) rotateSession(ctx context.Context, w http.ResponseWriter, a *authed) error {
	token, hash, err := newToken()
	if err != nil {
		return err
	}
	if err := s.store.RotateSession(ctx, a.tokenHash, hash); err != nil {
		return err
	}
	a.tokenHash = hash
	s.setSessionCookie(w, token, a.session.ExpiresAt)
	return nil
}

func (s *Server) setSessionCookie(w http.ResponseWriter, token string, expires time.Time) {
	s.setCookie(w, sessionCookie, token, max(1, int(time.Until(expires).Seconds())))
}

type authed struct {
	session   *store.Session
	tokenHash []byte
}

func (s *Server) session(r *http.Request) (*authed, error) {
	token := s.cookie(r, sessionCookie)
	if token == "" {
		return nil, fail(http.StatusUnauthorized, "please log in")
	}
	h := sha([]byte(token))
	sess, err := s.store.SessionByToken(r.Context(), h)
	if errors.Is(err, store.ErrNotFound) {
		return nil, fail(http.StatusUnauthorized, "your session has ended; please log in again")
	}
	if err != nil {
		return nil, err
	}
	return &authed{session: sess, tokenHash: h}, nil
}

func (s *Server) unlocked(r *http.Request) (*authed, error) {
	a, err := s.session(r)
	if err != nil {
		return nil, err
	}
	if a.session.UnlockStage != 3 {
		return nil, fail(http.StatusLocked, "answer your questions to unlock the vault")
	}
	return a, nil
}

func ParseProxies(list string) ([]netip.Prefix, error) {
	var out []netip.Prefix
	for _, part := range strings.Split(list, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if p, err := netip.ParsePrefix(part); err == nil {
			out = append(out, p.Masked())
			continue
		}
		a, err := netip.ParseAddr(part)
		if err != nil {
			return nil, fmt.Errorf("%q is not an address or a CIDR range", part)
		}
		out = append(out, netip.PrefixFrom(a.Unmap(), a.Unmap().BitLen()))
	}
	return out, nil
}

func (s *Server) trusted(addr string) bool {
	a, err := netip.ParseAddr(strings.TrimSpace(addr))
	if err != nil {
		return false
	}
	a = a.Unmap()
	for _, p := range s.cfg.TrustedProxies {
		if p.Contains(a) {
			return true
		}
	}
	return false
}

func peer(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (s *Server) fromProxy(r *http.Request) bool {
	return s.trusted(peer(r))
}

func (s *Server) clientIP(r *http.Request) string {
	ip := peer(r)
	if !s.fromProxy(r) {
		return ip
	}
	hops := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	for i := len(hops) - 1; i >= 0; i-- {
		hop := strings.TrimSpace(hops[i])
		if hop == "" {
			break
		}
		ip = hop
		if !s.trusted(hop) {
			break
		}
	}
	return ip
}

func limiterKey(ip string) string {
	a, err := netip.ParseAddr(ip)
	if err != nil || a.Unmap().Is4() {
		return ip
	}
	return netip.PrefixFrom(a, 64).Masked().String()
}

func newUUID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}
