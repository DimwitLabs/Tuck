package server

import (
	"log/slog"
	"net/http"
)

func (s *Server) audit(r *http.Request, event string, attrs ...any) {
	slog.Info("audit", append([]any{"event", event, "ip", s.clientIP(r)}, attrs...)...)
}
