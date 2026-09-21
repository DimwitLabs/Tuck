package store

import (
	"fmt"
	"net"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

func CheckTransport(databaseURL string) error {
	cfg, err := pgconn.ParseConfig(databaseURL)
	if err != nil {
		return fmt.Errorf("TUCK_DATABASE_URL: %w", err)
	}
	if localHost(cfg.Host) {
		return nil
	}
	plaintext := cfg.TLSConfig == nil
	for _, fb := range cfg.Fallbacks {
		if fb.TLSConfig == nil {
			plaintext = true
		}
	}
	if plaintext {
		return fmt.Errorf("the database at %s would be reached without TLS; add sslmode=require (or verify-full) to "+
			"TUCK_DATABASE_URL, or set TUCK_DB_ALLOW_PLAINTEXT=true if that network is private and trusted", cfg.Host)
	}
	return nil
}

func localHost(host string) bool {
	if strings.HasPrefix(host, "/") || host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
