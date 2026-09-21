package server

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/DimwitLabs/tuck/internal/store"
)

// Needs a real Postgres at TUCK_TEST_DATABASE_URL; each test gets its own schema.
func newTestServer(t *testing.T, opts ...func(*Config)) (*httptest.Server, string) {
	t.Helper()
	url := os.Getenv("TUCK_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TUCK_TEST_DATABASE_URL not set")
	}
	schema := fmt.Sprintf("tuck_test_%x", rnd(6))
	ctx := context.Background()
	st, err := store.Open(ctx, url, schema)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		conn, err := pgx.Connect(ctx, url)
		if err == nil {
			_, _ = conn.Exec(ctx, "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE")
			conn.Close(ctx)
		}
		st.Close()
	})
	static := fstest.MapFS{
		"index.html":    {Data: []byte("<!doctype html><title>Tuck</title>")},
		"assets/app.js": {Data: []byte("console.log(1)")},
	}
	cfg := Config{
		Features: FeaturesBoth, SessionTTL: time.Hour, MaxFileBytes: 1 << 10,
		Lockout: store.Lockout{Every: 5, Base: time.Minute, FreezeAfter: 2},
		Quota:   store.Quota{Items: 100, Bytes: 1 << 20},
		Secret:  rnd(32),
	}
	for _, o := range opts {
		o(&cfg)
	}
	srv := New(st, cfg, static)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts, schema
}

func rnd(n int) []byte {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return b
}

func b64s(b []byte) string { return base64.StdEncoding.EncodeToString(b) }

type account struct {
	username string
	authKey  []byte
	proofs   [3][]byte
}

func newAccount(name string) *account {
	a := &account{username: name, authKey: rnd(32)}
	for i := range a.proofs {
		a.proofs[i] = rnd(32)
	}
	return a
}

func (a *account) enrollment() map[string]any {
	steps := make([]map[string]string, 3)
	for i := range steps {
		steps[i] = map[string]string{
			"salt": b64s(rnd(16)), "questionNonce": b64s(rnd(12)),
			"questionCiphertext": b64s(rnd(40)), "proof": b64s(a.proofs[i]),
		}
	}
	return map[string]any{
		"kdf":     map[string]any{"algorithm": "argon2id", "iterations": 3, "memoryKiB": 65536, "parallelism": 1},
		"kdfSalt": b64s(rnd(16)), "authKey": b64s(a.authKey), "steps": steps,
		"wrappedKey": b64s(rnd(48)), "wrappedNonce": b64s(rnd(12)),
	}
}

type client struct {
	t    *testing.T
	base string
	http *http.Client
}

func newClient(t *testing.T, ts *httptest.Server) *client {
	jar, _ := cookiejar.New(nil)
	return &client{t: t, base: ts.URL, http: &http.Client{Jar: jar}}
}

func (c *client) do(method, path string, body any, csrf bool) (int, map[string]any, http.Header) {
	c.t.Helper()
	var rdr io.Reader
	contentType := "application/json"
	switch b := body.(type) {
	case nil:
	case []byte:
		rdr, contentType = bytes.NewReader(b), "application/octet-stream"
	default:
		raw, _ := json.Marshal(b)
		rdr = bytes.NewReader(raw)
	}
	req, _ := http.NewRequest(method, c.base+path, rdr)
	req.Header.Set("Content-Type", contentType)
	if csrf {
		req.Header.Set("X-Tuck-Request", "1")
	}
	res, err := c.http.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	out := map[string]any{}
	if json.Unmarshal(raw, &out) != nil {
		out["raw"] = raw
	}
	return res.StatusCode, out, res.Header
}

func (c *client) call(method, path string, body any) (int, map[string]any) {
	c.t.Helper()
	status, out, _ := c.do(method, path, body, true)
	return status, out
}

func expect(t *testing.T, got, want int, body map[string]any) {
	t.Helper()
	if got != want {
		t.Fatalf("status %d, want %d: %v", got, want, body)
	}
}

func (c *client) signup(a *account) {
	c.t.Helper()
	s, b := c.call("POST", "/api/signup", map[string]any{"username": a.username, "enrollment": a.enrollment()})
	expect(c.t, s, 201, b)
}

func (c *client) login(a *account) {
	c.t.Helper()
	s, b := c.call("POST", "/api/login", map[string]any{"username": a.username, "authKey": b64s(a.authKey)})
	expect(c.t, s, 200, b)
}

func (c *client) unlock(a *account) map[string]any {
	c.t.Helper()
	s, b := c.call("POST", "/api/unlock/start", nil)
	expect(c.t, s, 200, b)
	for i := 1; i <= 3; i++ {
		s, b = c.call("POST", "/api/unlock/answer", map[string]any{"step": i, "proof": b64s(a.proofs[i-1])})
		expect(c.t, s, 200, b)
	}
	return b
}

const itemA = "7f1c2a3b-4d5e-4f60-8a9b-0c1d2e3f4a5b"

func item(kind string) map[string]any {
	return map[string]any{"kind": kind, "nonce": b64s(rnd(12)), "ciphertext": b64s(rnd(64))}
}

func TestSignupStartsUnlockedAndAllowsOnlyOneAccount(t *testing.T) {
	ts, _ := newTestServer(t)
	c := newClient(t, ts)

	_, st := c.call("GET", "/api/status", nil)
	if st["signupOpen"] != true {
		t.Fatalf("fresh instance should be open: %v", st)
	}
	c.signup(newAccount("alex"))
	s, b := c.call("PUT", "/api/items/"+itemA, item("host"))
	expect(t, s, 204, b)

	_, st = c.call("GET", "/api/status", nil)
	if st["signupOpen"] != false || st["session"].(map[string]any)["unlocked"] != true {
		t.Fatalf("after first sign-up: %v", st)
	}
	s, b = newClient(t, ts).call("POST", "/api/signup", map[string]any{"username": "intruder", "enrollment": newAccount("intruder").enrollment()})
	expect(t, s, 403, b)
}

func TestUnlockIsOneQuestionAtATimeAndOnlyOnCorrectness(t *testing.T) {
	ts, _ := newTestServer(t)
	a := newAccount("alex")
	newClient(t, ts).signup(a)

	c := newClient(t, ts)
	s, b := c.call("POST", "/api/login", map[string]any{"username": "alex", "authKey": b64s(rnd(32))})
	expect(t, s, 401, b)
	c.login(a)

	s, b = c.call("GET", "/api/items", nil)
	expect(t, s, 423, b)

	s, b = c.call("POST", "/api/unlock/start", nil)
	expect(t, s, 200, b)
	if b["step"] != float64(1) || b["question"] == nil {
		t.Fatalf("start should hand out question 1: %v", b)
	}

	s, b = c.call("POST", "/api/unlock/answer", map[string]any{"step": 2, "proof": b64s(a.proofs[1])})
	expect(t, s, 409, b)

	s, b = c.call("POST", "/api/unlock/answer", map[string]any{"step": 1, "proof": b64s(rnd(32))})
	expect(t, s, 403, b)
	if b["attemptsBeforeLock"] != float64(4) {
		t.Fatalf("wrong answer should report remaining attempts: %v", b)
	}

	s, b = c.call("POST", "/api/unlock/answer", map[string]any{"step": 1, "proof": b64s(a.proofs[0])})
	expect(t, s, 200, b)
	if b["step"] != float64(2) || b["question"] == nil || b["wrappedKey"] != nil {
		t.Fatalf("correct answer 1 should reveal only question 2: %v", b)
	}
	s, b = c.call("GET", "/api/items", nil)
	expect(t, s, 423, b)

	s, b = c.call("POST", "/api/unlock/answer", map[string]any{"step": 2, "proof": b64s(a.proofs[1])})
	expect(t, s, 200, b)
	s, b = c.call("POST", "/api/unlock/answer", map[string]any{"step": 3, "proof": b64s(a.proofs[2])})
	expect(t, s, 200, b)
	if b["wrappedKey"] == nil || b["wrappedNonce"] == nil {
		t.Fatalf("final answer should release the wrapped key: %v", b)
	}
	s, b = c.call("GET", "/api/items", nil)
	expect(t, s, 200, b)

	s, b = c.call("POST", "/api/lock", nil)
	expect(t, s, 204, b)
	s, b = c.call("GET", "/api/items", nil)
	expect(t, s, 423, b)
}

func TestWrongAnswersLockOutEvenTheRightAnswer(t *testing.T) {
	ts, _ := newTestServer(t)
	a := newAccount("alex")
	newClient(t, ts).signup(a)
	c := newClient(t, ts)
	c.login(a)
	c.call("POST", "/api/unlock/start", nil)

	for i := 0; i < 4; i++ {
		s, b := c.call("POST", "/api/unlock/answer", map[string]any{"step": 1, "proof": b64s(rnd(32))})
		expect(t, s, 403, b)
	}
	s, b, h := c.do("POST", "/api/unlock/answer", map[string]any{"step": 1, "proof": b64s(rnd(32))}, true)
	expect(t, s, 429, b)
	if h.Get("Retry-After") == "" {
		t.Fatal("lockout should send Retry-After")
	}
	s, b = c.call("POST", "/api/unlock/answer", map[string]any{"step": 1, "proof": b64s(a.proofs[0])})
	expect(t, s, 429, b)
	s, b = c.call("POST", "/api/unlock/start", nil)
	expect(t, s, 429, b)
}

func TestPreloginDoesNotRevealWhichUsersExist(t *testing.T) {
	ts, _ := newTestServer(t)
	newClient(t, ts).signup(newAccount("alex"))
	c := newClient(t, ts)

	_, real := c.call("GET", "/api/prelogin?username=alex", nil)
	_, ghost1 := c.call("GET", "/api/prelogin?username=ghost", nil)
	_, ghost2 := c.call("GET", "/api/prelogin?username=ghost", nil)
	_, other := c.call("GET", "/api/prelogin?username=someone", nil)
	if ghost1["kdfSalt"] != ghost2["kdfSalt"] || ghost1["kdfSalt"] == other["kdfSalt"] || ghost1["kdfSalt"] == real["kdfSalt"] {
		t.Fatal("decoy salts must be stable per username and distinct")
	}
	for _, r := range []map[string]any{real, ghost1} {
		salt, _ := base64.StdEncoding.DecodeString(r["kdfSalt"].(string))
		if len(salt) != 16 || r["kdf"] == nil {
			t.Fatalf("prelogin shape: %v", r)
		}
	}
}

func TestItemKindIsFixed(t *testing.T) {
	ts, _ := newTestServer(t)
	c := newClient(t, ts)
	c.signup(newAccount("alex"))

	s, b := c.call("PUT", "/api/items/"+itemA, item("credential"))
	expect(t, s, 204, b)
	s, b = c.call("PUT", "/api/items/"+itemA, item("host"))
	expect(t, s, 409, b)
	s, b = c.call("PUT", "/api/items/NOT-A-UUID", item("host"))
	expect(t, s, 400, b)
}

func TestFeaturesDecideWhatCanBeSaved(t *testing.T) {
	for features, allowed := range map[Features][]string{
		FeaturesSSH:   {"credential", "host"},
		FeaturesFiles: {"file"},
		FeaturesBoth:  {"credential", "host", "file"},
	} {
		ts, _ := newTestServer(t, func(c *Config) { c.Features = features })
		c := newClient(t, ts)
		c.signup(newAccount("alex"))
		_, st := c.call("GET", "/api/status", nil)
		if st["features"] != string(features) {
			t.Fatalf("status should report %s: %v", features, st)
		}
		for i, kind := range []string{"credential", "host", "file"} {
			id := fmt.Sprintf("7f1c2a3b-4d5e-4f60-8a9b-0c1d2e3f4a5%d", i)
			s, b := c.call("PUT", "/api/items/"+id, item(kind))
			want := 403
			for _, k := range allowed {
				if k == kind {
					want = 204
				}
			}
			if s != want {
				t.Fatalf("%s instance saving a %s: got %d, want %d: %v", features, kind, s, want, b)
			}
		}
		if features == FeaturesSSH {
			s, b := c.call("PUT", "/api/items/"+itemA+"/file", rnd(100))
			expect(t, s, 403, b)
		}
	}
}

func TestFilesRoundTripWithinTheLimit(t *testing.T) {
	ts, _ := newTestServer(t)
	c := newClient(t, ts)
	c.signup(newAccount("alex"))

	blob := rnd(500)
	s, b := c.call("PUT", "/api/items/"+itemA+"/file", blob)
	expect(t, s, 404, b)
	c.call("PUT", "/api/items/"+itemA, item("file"))
	s, b = c.call("PUT", "/api/items/"+itemA+"/file", blob)
	expect(t, s, 204, b)
	s, b = c.call("GET", "/api/items/"+itemA+"/file", nil)
	expect(t, s, 200, b)
	if !bytes.Equal(b["raw"].([]byte), blob) {
		t.Fatal("file bytes changed in transit")
	}
	s, b = c.call("PUT", "/api/items/"+itemA+"/file", rnd(2<<10))
	expect(t, s, 413, b)

	_, list := c.call("GET", "/api/items", nil)
	if list["items"].([]any)[0].(map[string]any)["hasFile"] != true {
		t.Fatal("listing should flag items with content")
	}
}

func TestRekeyReplacesSecretsAndEndsOtherSessions(t *testing.T) {
	ts, _ := newTestServer(t)
	a := newAccount("alex")
	primary := newClient(t, ts)
	primary.signup(a)
	other := newClient(t, ts)
	other.login(a)

	next := newAccount("alex")
	s, b := primary.call("PUT", "/api/account/keys", map[string]any{"currentAuthKey": b64s(rnd(32)), "enrollment": next.enrollment()})
	expect(t, s, 403, b)
	s, b = primary.call("PUT", "/api/account/keys", map[string]any{"currentAuthKey": b64s(a.authKey), "enrollment": next.enrollment()})
	expect(t, s, 204, b)

	s, b = other.call("POST", "/api/unlock/start", nil)
	expect(t, s, 401, b)
	s, b = primary.call("GET", "/api/items", nil)
	expect(t, s, 200, b)

	fresh := newClient(t, ts)
	s, b = fresh.call("POST", "/api/login", map[string]any{"username": "alex", "authKey": b64s(a.authKey)})
	expect(t, s, 401, b)
	fresh.login(next)
	fresh.unlock(next)
}

func TestWeakKdfAndMalformedEnrollmentAreRefused(t *testing.T) {
	ts, _ := newTestServer(t)
	c := newClient(t, ts)
	e := newAccount("alex").enrollment()
	e["kdf"] = map[string]any{"algorithm": "argon2id", "iterations": 1, "memoryKiB": 1024, "parallelism": 1}
	s, b := c.call("POST", "/api/signup", map[string]any{"username": "alex", "enrollment": e})
	expect(t, s, 400, b)

	e = newAccount("alex").enrollment()
	e["steps"] = e["steps"].([]map[string]string)[:2]
	s, b = c.call("POST", "/api/signup", map[string]any{"username": "alex", "enrollment": e})
	expect(t, s, 400, b)

	s, b = c.call("POST", "/api/signup", map[string]any{"username": "no spaces!", "enrollment": newAccount("x").enrollment()})
	expect(t, s, 400, b)
}

func TestCsrfHeaderAndSecurityHeaders(t *testing.T) {
	ts, _ := newTestServer(t)
	c := newClient(t, ts)
	s, b, _ := c.do("POST", "/api/signup", map[string]any{"username": "alex", "enrollment": newAccount("d").enrollment()}, false)
	expect(t, s, 403, b)

	s, _, h := c.do("GET", "/some/client/route", nil, false)
	expect(t, s, 200, nil)
	if h.Get("Content-Security-Policy") == "" || h.Get("X-Frame-Options") != "DENY" {
		t.Fatalf("missing security headers: %v", h)
	}
	_, _, h = c.do("GET", "/api/status", nil, false)
	if h.Get("Cache-Control") != "no-store" {
		t.Fatal("API responses must not be cached")
	}
}

func TestSchemaIsConfigurable(t *testing.T) {
	_, schema := newTestServer(t)
	conn, err := pgx.Connect(context.Background(), os.Getenv("TUCK_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	var n int
	_ = conn.QueryRow(context.Background(),
		`SELECT count(*) FROM information_schema.tables WHERE table_schema = $1`, schema).Scan(&n)
	if n != 7 {
		t.Fatalf("expected 7 tables in %s, found %d", schema, n)
	}
}

func dbExec(t *testing.T, schema, sql string, args ...any) {
	t.Helper()
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, os.Getenv("TUCK_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err := conn.Exec(ctx, strings.ReplaceAll(sql, "{s}", pgx.Identifier{schema}.Sanitize()), args...); err != nil {
		t.Fatal(err)
	}
}

func dbCount(t *testing.T, schema, sql string) int {
	t.Helper()
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, os.Getenv("TUCK_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	var n int
	if err := conn.QueryRow(ctx, strings.ReplaceAll(sql, "{s}", pgx.Identifier{schema}.Sanitize())).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestParallelGuessesCannotOutrunTheLockout(t *testing.T) {
	ts, _ := newTestServer(t)
	a := newAccount("alex")
	newClient(t, ts).signup(a)
	c := newClient(t, ts)
	c.login(a)
	c.call("POST", "/api/unlock/start", nil)

	var wg sync.WaitGroup
	var mu sync.Mutex
	statuses := map[int]int{}
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, _ := c.call("POST", "/api/unlock/answer", map[string]any{"step": 1, "proof": b64s(rnd(32))})
			mu.Lock()
			statuses[s]++
			mu.Unlock()
		}()
	}
	wg.Wait()
	if statuses[403] != 4 || statuses[429] != 16 {
		t.Fatalf("expected 4 wrong answers then lockout, got %v", statuses)
	}
}

func TestEachLockoutDoublesTheLast(t *testing.T) {
	ts, schema := newTestServer(t)
	a := newAccount("alex")
	newClient(t, ts).signup(a)
	c := newClient(t, ts)
	c.login(a)
	c.call("POST", "/api/unlock/start", nil)

	retryAfter := func() int {
		var h http.Header
		for i := 0; i < 5; i++ {
			_, _, h = c.do("POST", "/api/unlock/answer", map[string]any{"step": 1, "proof": b64s(rnd(32))}, true)
		}
		n, _ := strconv.Atoi(h.Get("Retry-After"))
		return n
	}
	first := retryAfter()
	dbExec(t, schema, `UPDATE {s}.users SET unlock_locked_until = now() - interval '1 second'`)
	second := retryAfter()
	if first < 55 || first > 61 || second < 115 || second > 121 {
		t.Fatalf("lockouts should be 1m then 2m, got %ds then %ds", first, second)
	}

	dbExec(t, schema, `UPDATE {s}.users SET unlock_locked_until = NULL`)
	c.unlock(a)
	if n := dbCount(t, schema, `SELECT unlock_lockouts FROM {s}.users`); n != 0 {
		t.Fatalf("a full unlock should reset the escalation, got %d", n)
	}
}

func TestProxyHeadersCannotBeSpoofed(t *testing.T) {
	signups := func(proxies ...string) []int {
		ts, _ := newTestServer(t, func(c *Config) {
			for _, p := range proxies {
				c.TrustedProxies = append(c.TrustedProxies, netip.MustParsePrefix(p))
			}
		})
		statuses := []int{}
		for i := 0; i < 6; i++ {
			c := newClient(t, ts)
			req, _ := http.NewRequest("POST", ts.URL+"/api/signup", nil)
			raw, _ := json.Marshal(map[string]any{"username": fmt.Sprintf("user%d", i), "enrollment": newAccount("x").enrollment()})
			req.Body = io.NopCloser(bytes.NewReader(raw))
			req.Header.Set("X-Tuck-Request", "1")
			req.Header.Set("X-Real-IP", fmt.Sprintf("203.0.113.%d", i))
			req.Header.Set("X-Forwarded-For", fmt.Sprintf("198.51.100.%d, 10.0.0.1", i))
			res, err := c.http.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			res.Body.Close()
			statuses = append(statuses, res.StatusCode)
		}
		return statuses
	}
	if s := signups(); s[5] != 429 {
		t.Fatalf("with no trusted proxy, forwarded headers must be ignored, got %v", s)
	}
	if s := signups("10.9.9.9/32"); s[5] != 429 {
		t.Fatalf("headers from a peer that is not a trusted proxy must be ignored, got %v", s)
	}
	if s := signups("127.0.0.1/32"); s[5] != 429 {
		t.Fatalf("only the trusted proxy's own hop should count, got %v", s)
	}
	if s := signups("127.0.0.1/32", "10.0.0.0/8"); s[5] == 429 {
		t.Fatalf("a chain of trusted proxies should reach the real client, got %v", s)
	}
}

func TestParseProxies(t *testing.T) {
	got, err := ParseProxies(" 127.0.0.1, 172.18.0.5/16,::1 ,")
	if err != nil || fmt.Sprint(got) != "[127.0.0.1/32 172.18.0.0/16 ::1/128]" {
		t.Fatalf("got %v %v", got, err)
	}
	if _, err := ParseProxies("true"); err == nil {
		t.Fatal("a stray boolean should be refused")
	}
}

func TestSessionTokenRotatesOnUnlockAndRekey(t *testing.T) {
	ts, schema := newTestServer(t)
	a := newAccount("alex")
	newClient(t, ts).signup(a)
	c := newClient(t, ts)
	c.login(a)
	base, _ := url.Parse(ts.URL)
	before := c.http.Jar.Cookies(base)
	expires := dbCount(t, schema, `SELECT count(DISTINCT expires_at) FROM {s}.sessions`)
	c.unlock(a)
	after := c.http.Jar.Cookies(base)
	if fmt.Sprint(before) == fmt.Sprint(after) {
		t.Fatal("unlocking should issue a new session token")
	}
	if n := dbCount(t, schema, `SELECT count(DISTINCT expires_at) FROM {s}.sessions`); n != expires {
		t.Fatal("rotating must not change when a session expires")
	}

	stale := newClient(t, ts)
	stale.http.Jar.SetCookies(base, before)
	s, b := stale.call("GET", "/api/items", nil)
	expect(t, s, 401, b)
	s, b = c.call("GET", "/api/items", nil)
	expect(t, s, 200, b)

	unlocked := c.http.Jar.Cookies(base)
	s, b = c.call("PUT", "/api/account/keys", map[string]any{"currentAuthKey": b64s(a.authKey), "enrollment": newAccount("alex").enrollment()})
	expect(t, s, 204, b)
	stale.http.Jar.SetCookies(base, unlocked)
	s, b = stale.call("GET", "/api/items", nil)
	expect(t, s, 401, b)
	s, b = c.call("GET", "/api/items", nil)
	expect(t, s, 200, b)
}

func TestLoginReplacesTheSessionItCameFrom(t *testing.T) {
	ts, schema := newTestServer(t)
	a := newAccount("alex")
	newClient(t, ts).signup(a)
	c := newClient(t, ts)
	c.login(a)
	c.login(a)
	c.login(a)
	if n := dbCount(t, schema, `SELECT count(*) FROM {s}.sessions`); n != 2 {
		t.Fatalf("expected the sign-up session and one login session, got %d", n)
	}
}

func TestEdgeCasesGetTheRightStatus(t *testing.T) {
	ts, _ := newTestServer(t)
	c := newClient(t, ts)
	c.signup(newAccount("alex"))

	tiny := map[string]any{"kind": "host", "nonce": b64s(rnd(12)), "ciphertext": b64s(rnd(8))}
	s, b := c.call("PUT", "/api/items/"+itemA, tiny)
	expect(t, s, 400, b)

	_, st := c.call("GET", "/api/status", nil)
	if st["maxFileBytes"] != float64(1<<10) {
		t.Fatalf("status should report the file limit: %v", st)
	}

	s, b, _ = c.do("GET", "/assets", nil, false)
	expect(t, s, 200, b)
	if !strings.Contains(string(b["raw"].([]byte)), "<title>Tuck</title>") {
		t.Fatalf("a directory should fall back to the app, not list its files: %s", b["raw"])
	}
}

func TestTheLastLockoutFreezesUntilTheDatabaseLiftsIt(t *testing.T) {
	ts, schema := newTestServer(t)
	a := newAccount("alex")
	newClient(t, ts).signup(a)
	c := newClient(t, ts)
	c.login(a)
	c.call("POST", "/api/unlock/start", nil)

	wrong := func() (int, map[string]any) {
		return c.call("POST", "/api/unlock/answer", map[string]any{"step": 1, "proof": b64s(rnd(32))})
	}
	expire := func() { dbExec(t, schema, `UPDATE {s}.users SET unlock_locked_until = now() - interval '1 second'`) }

	for round := 0; round < 2; round++ {
		for i := 0; i < 4; i++ {
			_, b := wrong()
			if b["freezesNext"] != false {
				t.Fatalf("timed round %d should not warn about freezing: %v", round, b)
			}
		}
		s, b := wrong()
		expect(t, s, 429, b)
		expire()
	}
	for i := 0; i < 4; i++ {
		if _, b := wrong(); b["freezesNext"] != true {
			t.Fatalf("the round before a freeze should warn: %v", b)
		}
	}
	s, b := wrong()
	expect(t, s, 423, b)
	if b["frozen"] != true {
		t.Fatalf("third lockout should freeze: %v", b)
	}

	expire()
	s, b = c.call("POST", "/api/unlock/answer", map[string]any{"step": 1, "proof": b64s(a.proofs[0])})
	expect(t, s, 423, b)
	s, b = c.call("POST", "/api/unlock/start", nil)
	expect(t, s, 423, b)

	dbExec(t, schema, `UPDATE {s}.users SET unlock_frozen = false, failed_unlocks = 0, unlock_lockouts = 0, unlock_locked_until = NULL`)
	c.unlock(a)
}

func TestGateKeepsTheFrontDoorShut(t *testing.T) {
	ts, _ := newTestServer(t, func(c *Config) { c.GatePassword = "open sesame" })
	c := newClient(t, ts)
	a := newAccount("alex")

	_, st := c.call("GET", "/api/status", nil)
	if st["gate"] != true || st["signupOpen"] != nil {
		t.Fatalf("a closed gate should reveal nothing else: %v", st)
	}
	s, b := c.call("GET", "/api/prelogin?username=alex", nil)
	expect(t, s, 403, b)
	s, b = c.call("POST", "/api/signup", map[string]any{"username": a.username, "enrollment": a.enrollment()})
	expect(t, s, 403, b)
	for _, typed := range []string{"open sesam", "open sesame?", "open sesame please", "OPEN SESAME"} {
		_, b = c.call("POST", "/api/gate", map[string]any{"typed": typed})
		if b["open"] != false {
			t.Fatalf("%q should not open the gate: %v", typed, b)
		}
	}
	_, b = c.call("POST", "/api/gate", map[string]any{"typed": "try try hmm okay open sesame"})
	if b["open"] != true {
		t.Fatalf("the password at the end of other typing should open the gate: %v", b)
	}
	_, st = c.call("GET", "/api/status", nil)
	if st["gate"] != false || st["signupOpen"] != true {
		t.Fatalf("an open gate should show the usual status: %v", st)
	}
	c.signup(a)

	s, b = c.call("POST", "/api/logout", nil)
	expect(t, s, 204, b)
	s, b = c.call("POST", "/api/login", map[string]any{"username": a.username, "authKey": b64s(a.authKey)})
	expect(t, s, 403, b)

	other := newClient(t, ts)
	other.call("POST", "/api/gate", map[string]any{"typed": "open sesame"})
	other.login(a)
	other.unlock(a)
}

func TestGatePassesAreSignedAndExpire(t *testing.T) {
	ts, _ := newTestServer(t, func(c *Config) { c.GatePassword, c.GateWord = "open sesame", "hello." })
	forged := fmt.Sprintf("%d.%s", time.Now().Add(time.Hour).Unix(), strings.Repeat("ab", 32))
	req, _ := http.NewRequest("GET", ts.URL+"/api/status", nil)
	req.AddCookie(&http.Cookie{Name: gateCookie, Value: forged})
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var st map[string]any
	_ = json.NewDecoder(res.Body).Decode(&st)
	if st["gate"] != true || st["word"] != "hello." {
		t.Fatalf("a forged pass must not open the gate, which shows only its word: %v", st)
	}

	page, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer page.Body.Close()
	body, _ := io.ReadAll(page.Body)
	if !strings.Contains(string(body), "<title>hello.</title>") || strings.Contains(string(body), "Tuck") {
		t.Fatalf("behind a gate the page should be titled with the gate word: %s", body)
	}
}

func TestPlainHTTPIsRefusedWhenCookiesAreSecure(t *testing.T) {
	get := func(ts *httptest.Server, path string, headers map[string]string) *http.Response {
		req, _ := http.NewRequest("GET", ts.URL+path, nil)
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res
	}

	direct, _ := newTestServer(t, func(c *Config) { c.SecureCookies = true })
	if res := get(direct, "/api/status", nil); res.StatusCode != 403 {
		t.Fatalf("plain http should be refused, got %d", res.StatusCode)
	}
	if res := get(direct, "/", map[string]string{"X-Forwarded-Proto": "https"}); res.StatusCode != 403 {
		t.Fatalf("X-Forwarded-Proto must be ignored without a trusted proxy, got %d", res.StatusCode)
	}
	if res := get(direct, "/api/health", nil); res.StatusCode != 200 {
		t.Fatalf("the container's own health check must still work over http, got %d", res.StatusCode)
	}

	elsewhere, _ := newTestServer(t, func(c *Config) {
		c.SecureCookies, c.TrustedProxies = true, []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
	})
	if res := get(elsewhere, "/", map[string]string{"X-Forwarded-Proto": "https"}); res.StatusCode != 403 {
		t.Fatalf("X-Forwarded-Proto must be ignored from a peer that is not a trusted proxy, got %d", res.StatusCode)
	}

	proxied, _ := newTestServer(t, func(c *Config) {
		c.SecureCookies, c.TrustedProxies = true, []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")}
	})
	res := get(proxied, "/api/status", map[string]string{"X-Forwarded-Proto": "https"})
	if res.StatusCode != 200 || res.Header.Get("Strict-Transport-Security") == "" {
		t.Fatalf("https through a trusted proxy should work and send HSTS: %d %v", res.StatusCode, res.Header)
	}
	if res := get(proxied, "/api/status", map[string]string{"X-Forwarded-Proto": "https, http"}); res.StatusCode != 403 {
		t.Fatalf("only the proxy's own hop counts, got %d", res.StatusCode)
	}
}

func TestQuotasCapItemsAndBytes(t *testing.T) {
	ts, _ := newTestServer(t, func(c *Config) { c.Quota = store.Quota{Items: 2, Bytes: 1100} })
	c := newClient(t, ts)
	c.signup(newAccount("alex"))
	ids := []string{itemA, "7f1c2a3b-4d5e-4f60-8a9b-0c1d2e3f4a5c", "7f1c2a3b-4d5e-4f60-8a9b-0c1d2e3f4a5d"}

	s, b := c.call("PUT", "/api/items/"+ids[0], item("file"))
	expect(t, s, 204, b)
	s, b = c.call("PUT", "/api/items/"+ids[1], item("host"))
	expect(t, s, 204, b)
	s, b = c.call("PUT", "/api/items/"+ids[2], item("host"))
	expect(t, s, 507, b)
	s, b = c.call("PUT", "/api/items/"+ids[1], item("host"))
	expect(t, s, 204, b)

	s, b = c.call("PUT", "/api/items/"+ids[0]+"/file", rnd(900))
	expect(t, s, 204, b)
	s, b = c.call("PUT", "/api/items/"+ids[0]+"/file", rnd(950))
	expect(t, s, 204, b)
	s, b = c.call("PUT", "/api/items/"+ids[0]+"/file", rnd(1020))
	expect(t, s, 507, b)
}

func TestStrangersCannotLockTheOwnerOut(t *testing.T) {
	ts, _ := newTestServer(t)
	a := newAccount("alex")
	owner := newClient(t, ts)
	owner.signup(a)
	owner.call("POST", "/api/logout", nil)

	stranger := newClient(t, ts)
	last := 0
	for i := 0; i < 11; i++ {
		last, _ = stranger.call("POST", "/api/login", map[string]any{"username": "alex", "authKey": b64s(rnd(32))})
	}
	if last != 429 {
		t.Fatalf("a stranger should run out of tries, got %d", last)
	}
	s, b := stranger.call("POST", "/api/login", map[string]any{"username": "alex", "authKey": b64s(a.authKey)})
	expect(t, s, 429, b)
	owner.login(a)

	s, b = stranger.call("POST", "/api/login", map[string]any{"username": strings.Repeat("x", 100_000), "authKey": b64s(rnd(32))})
	expect(t, s, 401, b)
}

func TestWrongPasswordsAtRekeyEndTheSession(t *testing.T) {
	ts, _ := newTestServer(t)
	a := newAccount("alex")
	c := newClient(t, ts)
	c.signup(a)
	for i := 0; i < 5; i++ {
		s, b := c.call("PUT", "/api/account/keys", map[string]any{"currentAuthKey": b64s(rnd(32)), "enrollment": newAccount("alex").enrollment()})
		expect(t, s, 403, b)
	}
	s, b := c.call("PUT", "/api/account/keys", map[string]any{"currentAuthKey": b64s(rnd(32)), "enrollment": newAccount("alex").enrollment()})
	expect(t, s, 401, b)
	s, b = c.call("GET", "/api/items", nil)
	expect(t, s, 401, b)
}

func TestHealthDoesNotShowTheVersion(t *testing.T) {
	ts, _ := newTestServer(t)
	s, b := newClient(t, ts).call("GET", "/api/health", nil)
	expect(t, s, 200, b)
	if _, ok := b["version"]; ok {
		t.Fatalf("health should not name the version: %v", b)
	}
}

func TestSecureCookiesUseTheHostPrefix(t *testing.T) {
	secure := &Server{cfg: Config{SecureCookies: true}}
	plain := &Server{cfg: Config{}}
	if secure.cookieName(sessionCookie) != "__Host-tuck_session" || plain.cookieName(sessionCookie) != "tuck_session" {
		t.Fatal("secure cookies should carry __Host-")
	}
}

func TestIPv6ClientsAreLimitedPerSlash64(t *testing.T) {
	if limiterKey("2001:db8:1:2:aaaa::1") != limiterKey("2001:db8:1:2:bbbb::9") {
		t.Fatal("addresses in one /64 should share a limit")
	}
	if limiterKey("2001:db8:1:3::1") == limiterKey("2001:db8:1:2::1") || limiterKey("203.0.113.9") != "203.0.113.9" {
		t.Fatal("different /64s and IPv4 addresses keep their own limits")
	}
}

func TestConditionalWritesRefuseStaleCopies(t *testing.T) {
	ts, _ := newTestServer(t)
	c := newClient(t, ts)
	c.signup(newAccount("alex"))
	put := func(ciphertext []byte, replaces any) (int, map[string]any) {
		return c.call("PUT", "/api/items/"+itemA, map[string]any{"kind": "manifest", "nonce": b64s(rnd(12)), "ciphertext": b64s(ciphertext), "replaces": replaces})
	}
	first, second := rnd(40), rnd(40)
	s, b := put(first, "")
	expect(t, s, 204, b)
	s, b = put(second, "")
	expect(t, s, 412, b)
	s, b = put(second, b64s(sha(rnd(40))))
	expect(t, s, 412, b)
	s, b = put(second, b64s(sha(first)))
	expect(t, s, 204, b)
	s, b = put(rnd(40), b64s(sha(first)))
	expect(t, s, 412, b)
}

func TestPublicRoutesRefuseWhenFull(t *testing.T) {
	s := &Server{public: make(chan struct{}, 1)}
	ok := s.bounded(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	call := func() int {
		rec := httptest.NewRecorder()
		ok(rec, httptest.NewRequest("GET", "/api/status", nil))
		return rec.Code
	}
	s.public <- struct{}{}
	if got := call(); got != http.StatusServiceUnavailable {
		t.Fatalf("a full server should refuse at once, got %d", got)
	}
	<-s.public
	if got := call(); got != http.StatusOK {
		t.Fatalf("a free slot should serve, got %d", got)
	}
	if len(s.public) != 0 {
		t.Fatal("a finished request must give its slot back")
	}
}
