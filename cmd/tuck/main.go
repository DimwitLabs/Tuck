package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/DimwitLabs/tuck/internal/server"
	"github.com/DimwitLabs/tuck/internal/store"
	"github.com/DimwitLabs/tuck/web"
)

var version = "dev"

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) (bool, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("%s must be true or false, not %q", key, v)
	}
	return b, nil
}

func envInt(key string, fallback int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("%s must be a positive whole number, not %q", key, v)
	}
	return n, nil
}

func run() error {
	databaseURL := os.Getenv("TUCK_DATABASE_URL")
	if databaseURL == "" {
		return errors.New("TUCK_DATABASE_URL is required, e.g. postgres://tuck:secret@db:5432/tuck")
	}
	features := server.Features(env("TUCK_FEATURES", string(server.FeaturesBoth)))
	if features != server.FeaturesBoth && features != server.FeaturesSSH && features != server.FeaturesFiles {
		return fmt.Errorf("TUCK_FEATURES must be both, ssh or files, not %q", features)
	}

	secureCookies, err := envBool("TUCK_SECURE_COOKIES", true)
	if err != nil {
		return err
	}
	trustedProxies, err := server.ParseProxies(os.Getenv("TUCK_TRUSTED_PROXIES"))
	if err != nil {
		return fmt.Errorf("TUCK_TRUSTED_PROXIES: %w", err)
	}
	sessionHours, err := envInt("TUCK_SESSION_HOURS", 12)
	if err != nil {
		return err
	}
	maxFileMB, err := envInt("TUCK_MAX_FILE_MB", 25)
	if err != nil {
		return err
	}
	freezeAfter, err := envInt("TUCK_FREEZE_AFTER", 6)
	if err != nil {
		return err
	}
	maxItems, err := envInt("TUCK_MAX_ITEMS", 5000)
	if err != nil {
		return err
	}
	maxStorageMB, err := envInt("TUCK_MAX_STORAGE_MB", 1024)
	if err != nil {
		return err
	}
	gateWord := env("TUCK_GATE_WORD", "tuck.")
	if n := utf8.RuneCountInString(gateWord); n > 64 || strings.IndexFunc(gateWord, unicode.IsControl) >= 0 {
		return errors.New("TUCK_GATE_WORD must be one line of at most 64 characters")
	}
	allowPlaintext, err := envBool("TUCK_DB_ALLOW_PLAINTEXT", false)
	if err != nil {
		return err
	}
	if !allowPlaintext {
		if err := store.CheckTransport(databaseURL); err != nil {
			return err
		}
	}
	if !secureCookies {
		slog.Warn("TUCK_SECURE_COOKIES=false: tuck is answering plain http with insecure cookies; use this for local testing only")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	schema := env("TUCK_DB_SCHEMA", "tuck")
	st, err := store.Open(ctx, databaseURL, schema)
	if err != nil {
		return err
	}
	defer st.Close()
	secret, err := st.Secret(ctx, "server")
	if err != nil {
		return err
	}

	srv := server.New(st, server.Config{
		Features:       features,
		SecureCookies:  secureCookies,
		TrustedProxies: trustedProxies,
		SessionTTL:     time.Duration(sessionHours) * time.Hour,
		MaxFileBytes:   int64(maxFileMB) << 20,
		Quota:          store.Quota{Items: maxItems, Bytes: int64(maxStorageMB) << 20},
		Lockout:        store.Lockout{Every: 5, Base: 15 * time.Minute, FreezeAfter: freezeAfter},
		Secret:         secret,
		GatePassword:   os.Getenv("TUCK_GATE_PASSWORD"),
		GateWord:       gateWord,
	}, web.Dist())
	go srv.Janitor(ctx)

	addr := env("TUCK_ADDR", ":8080")
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       2 * time.Minute,
		WriteTimeout:      2 * time.Minute,
		IdleTimeout:       2 * time.Minute,
	}
	errs := make(chan error, 1)
	go func() {
		slog.Info("tuck listening", "addr", addr, "schema", schema, "features", features, "version", version)
		errs <- httpServer.ListenAndServe()
	}()

	select {
	case err := <-errs:
		return err
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return httpServer.Shutdown(shutdown)
}

func healthcheck() int {
	addr := env("TUCK_ADDR", ":8080")
	if strings.HasPrefix(addr, ":") {
		addr = "localhost" + addr
	}
	client := &http.Client{Timeout: 3 * time.Second}
	res, err := client.Get("http://" + addr + "/api/health")
	if err != nil {
		return 1
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}

func assets() int {
	err := fs.WalkDir(web.Dist(), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(web.Dist(), path)
		if err != nil {
			return err
		}
		fmt.Printf("%x  %s\n", sha256.Sum256(b), path)
		return nil
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(healthcheck())
	}
	if len(os.Args) > 1 && os.Args[1] == "assets" {
		os.Exit(assets())
	}
	if err := run(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("tuck stopped", "err", err)
		os.Exit(1)
	}
}
