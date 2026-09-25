package server

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func withOrigin(origin string) func(*Config) {
	return func(c *Config) { c.Origin = origin }
}

func TestNewWebAuthnAcceptsOnlyANamedHTTPSAddress(t *testing.T) {
	for origin, want := range map[string]string{
		"https://tuck.example.com":      "tuck.example.com",
		"https://tuck.example.com:8443": "tuck.example.com",
		"http://localhost:8484":         "localhost",
		"https://localhost":             "localhost",
		"":                              "",
		"http://tuck.example.com":       "",
		"https://192.168.1.4:8443":      "",
		"http://127.0.0.1:8484":         "",
		"https://[::1]":                 "",
		"not a url":                     "",
		"https://":                      "",
	} {
		auth, err := newWebAuthn(origin, "tuck.")
		switch {
		case want == "" && origin == "":
			if auth != nil || err != nil {
				t.Errorf("%q: passkeys should stay off, got %v %v", origin, auth, err)
			}
		case want == "":
			if err == nil {
				t.Errorf("%q: should be refused", origin)
			}
		case err != nil:
			t.Errorf("%q: %v", origin, err)
		case auth.Config.RPID != want:
			t.Errorf("%q: rpid %q, want %q", origin, auth.Config.RPID, want)
		}
	}
}

func TestNewWebAuthnNamesItselfAfterTheDoorWord(t *testing.T) {
	auth, err := newWebAuthn("https://tuck.example.com", "")
	if err != nil {
		t.Fatal(err)
	}
	if auth.Config.RPDisplayName != "tuck.example.com" {
		t.Fatalf("no door word should fall back to the hostname, got %q", auth.Config.RPDisplayName)
	}
	auth, err = newWebAuthn("https://tuck.example.com", "mango.")
	if err != nil {
		t.Fatal(err)
	}
	if auth.Config.RPDisplayName != "mango." {
		t.Fatalf("display name %q, want the door word", auth.Config.RPDisplayName)
	}
}

func TestDeviceLabelIsTrimmedAndCutByRunes(t *testing.T) {
	long := strings.Repeat("é", labelMax+10)
	for raw, want := range map[string]string{
		"":                       "this device",
		"   ":                    "this device",
		"  this mac  ":           "this mac",
		"this iphone":            "this iphone",
		long:                     strings.Repeat("é", labelMax),
		strings.Repeat("a", 100): strings.Repeat("a", labelMax),
	} {
		got := deviceLabel(raw)
		if got != want {
			t.Errorf("deviceLabel(%q) = %q, want %q", raw, got, want)
		}
		if !utf8.ValidString(got) {
			t.Errorf("deviceLabel(%q) produced invalid utf-8", raw)
		}
	}
}

func utf8Valid(s string) bool {
	for _, r := range s {
		if r == '�' {
			return false
		}
	}
	return true
}

func TestSealedRoundTrips(t *testing.T) {
	s := &Server{cfg: Config{Secret: rnd(32)}}
	payload := rnd(64)
	got, ok := s.unsealed("add", s.sealed("add", payload, time.Minute))
	if !ok || string(got) != string(payload) {
		t.Fatalf("round trip failed: ok=%v", ok)
	}
}

func TestUnsealedRefusesAnythingTouched(t *testing.T) {
	s := &Server{cfg: Config{Secret: rnd(32)}}
	other := &Server{cfg: Config{Secret: rnd(32)}}
	good := s.sealed("add", []byte("challenge"), time.Minute)
	parts := strings.SplitN(good, ".", 3)

	stale := s.sealed("add", []byte("challenge"), -time.Second)
	body := base64.RawURLEncoding.EncodeToString([]byte("evil"))
	expires := strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10)

	for name, value := range map[string]string{
		"empty":               "",
		"no separator":        "justonepart",
		"one separator":       parts[0] + "." + parts[1],
		"expired":             stale,
		"expiry not a number": "soon." + parts[1] + "." + parts[2],
		"body swapped":        parts[0] + "." + body + "." + parts[2],
		"signature swapped":   parts[0] + "." + parts[1] + "." + hex.EncodeToString(rnd(32)),
		"signature not hex":   parts[0] + "." + parts[1] + ".zzzz",
		"expiry extended":     expires + "." + parts[1] + "." + parts[2],
		"body not base64":     parts[0] + ".!!!." + hex.EncodeToString(s.mac("add|!!!", parts[0])),
		"another purpose":     s.sealed("gate", []byte("challenge"), time.Minute),
		"another secret":      other.sealed("add", []byte("challenge"), time.Minute),
	} {
		if _, ok := s.unsealed("add", value); ok {
			t.Errorf("%s: should have been refused", name)
		}
	}
}

func TestPasskeyRoutesAreOffWithoutAnOrigin(t *testing.T) {
	ts, _ := newTestServer(t)
	c := newClient(t, ts)
	a := newAccount("alex")
	c.signup(a)

	for _, call := range []struct {
		method, path string
	}{
		{"POST", "/api/passkeys/start"},
		{"POST", "/api/passkeys/finish"},
		{"POST", "/api/gate/passkey/start"},
		{"POST", "/api/gate/passkey/finish"},
	} {
		s, b := c.call(call.method, call.path, map[string]any{})
		expect(t, s, 404, b)
	}

	s, b := c.call("GET", "/api/passkeys", nil)
	expect(t, s, 200, b)
	if b["enabled"] != false {
		t.Fatalf("passkeys should report themselves off: %v", b)
	}
}

func TestPasskeyListNeedsAnUnlockedVault(t *testing.T) {
	ts, _ := newTestServer(t, withOrigin("https://tuck.example.com"))
	c := newClient(t, ts)
	c.signup(newAccount("alex"))
	s, b := c.call("POST", "/api/lock", nil)
	expect(t, s, 204, b)

	for _, call := range []struct {
		method, path string
	}{
		{"GET", "/api/passkeys"},
		{"POST", "/api/passkeys/start"},
	} {
		s, b := c.call(call.method, call.path, nil)
		expect(t, s, 423, b)
	}
}

func TestPasskeyRegistrationOptionsAskForThisDevice(t *testing.T) {
	ts, _ := newTestServer(t, withOrigin("https://tuck.example.com"))
	c := newClient(t, ts)
	c.signup(newAccount("alex"))

	s, b := c.call("GET", "/api/passkeys", nil)
	expect(t, s, 200, b)
	if b["enabled"] != true {
		t.Fatalf("passkeys should be on: %v", b)
	}
	if b["userId"] == "" || b["userId"] == nil {
		t.Fatalf("the list must carry the user handle for signalAllAcceptedCredentials: %v", b)
	}
	if rows, ok := b["passkeys"].([]any); !ok || len(rows) != 0 {
		t.Fatalf("a fresh account has no passkeys: %v", b)
	}

	s, b = c.call("POST", "/api/passkeys/start", nil)
	expect(t, s, 200, b)
	rp, _ := b["rp"].(map[string]any)
	if rp["id"] != "tuck.example.com" {
		t.Fatalf("rp id %v, want the configured host", rp["id"])
	}
	if b["challenge"] == nil || b["challenge"] == "" {
		t.Fatalf("no challenge: %v", b)
	}
	hints, _ := b["hints"].([]any)
	if len(hints) != 1 || hints[0] != "client-device" {
		t.Fatalf("hints %v, want [client-device] so the browser offers its own sensor", b["hints"])
	}
	sel, _ := b["authenticatorSelection"].(map[string]any)
	if sel["authenticatorAttachment"] != "platform" || sel["residentKey"] != "required" || sel["userVerification"] != "required" {
		t.Fatalf("authenticator selection %v", sel)
	}
}

func TestPasskeyRegistrationRefusesRubbishAndStaleCeremonies(t *testing.T) {
	ts, _ := newTestServer(t, withOrigin("https://tuck.example.com"))
	c := newClient(t, ts)
	c.signup(newAccount("alex"))
	rubbish := map[string]any{"id": "nope"}

	t.Run("no ceremony was ever started", func(t *testing.T) {
		s, b := c.call("POST", "/api/passkeys/finish", rubbish)
		expect(t, s, 400, b)
	})

	t.Run("a started ceremony still checks the credential", func(t *testing.T) {
		s, b := c.call("POST", "/api/passkeys/start", nil)
		expect(t, s, 200, b)
		if c.cookie(registerCookie) == "" {
			t.Fatal("starting a ceremony should hand back a sealed challenge")
		}
		s, b = c.call("POST", "/api/passkeys/finish", rubbish)
		expect(t, s, 400, b)
	})

	t.Run("the challenge is spent even when the answer was rubbish", func(t *testing.T) {
		if left := c.cookie(registerCookie); left != "" {
			t.Fatalf("the ceremony cookie should be cleared, still holds %q", left)
		}
		s, b := c.call("POST", "/api/passkeys/finish", rubbish)
		expect(t, s, 400, b)
	})
}

func TestVaultHoldsOnlySoManyPasskeys(t *testing.T) {
	ts, schema := newTestServer(t, withOrigin("https://tuck.example.com"))
	c := newClient(t, ts)
	c.signup(newAccount("alex"))

	_, b := c.call("GET", "/api/passkeys", nil)
	userID := decodeHandle(t, b["userId"].(string))
	for i := 0; i < maxPasskeys; i++ {
		dbExec(t, schema, fmt.Sprintf(
			`INSERT INTO %s.passkeys (credential_id, user_id, public_key, aaguid, transports, label)
			 VALUES ($1, $2, $3, $4, $5, $6)`, schema),
			rnd(16), userID, rnd(32), rnd(16), []string{"internal"}, fmt.Sprintf("device %d", i))
	}

	s, b := c.call("GET", "/api/passkeys", nil)
	expect(t, s, 200, b)
	rows, _ := b["passkeys"].([]any)
	if len(rows) != maxPasskeys {
		t.Fatalf("expected %d passkeys, got %d", maxPasskeys, len(rows))
	}
	first, _ := rows[0].(map[string]any)
	if first["label"] != "device 0" || first["id"] == "" || first["added"] == nil {
		t.Fatalf("row looks wrong: %v", first)
	}
	if first["lastUsed"] != nil {
		t.Fatalf("a passkey that never opened the door has no last use: %v", first)
	}

	s, b = c.call("POST", "/api/passkeys/start", nil)
	expect(t, s, 409, b)

	s, b = c.call("DELETE", "/api/passkeys/"+first["id"].(string), nil)
	expect(t, s, 200, b)
	rows, _ = b["passkeys"].([]any)
	if len(rows) != maxPasskeys-1 {
		t.Fatalf("forgetting one should leave %d, got %d", maxPasskeys-1, len(rows))
	}

	s, b = c.call("DELETE", "/api/passkeys/"+first["id"].(string), nil)
	expect(t, s, 404, b)

	s, b = c.call("DELETE", "/api/passkeys/not-base64-@@@", nil)
	expect(t, s, 400, b)
}

func decodeHandle(t *testing.T, handle string) string {
	t.Helper()
	raw, err := base64.RawURLEncoding.DecodeString(handle)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestDoorPasskeyStartOffersDiscoverableLogin(t *testing.T) {
	ts, _ := newTestServer(t, withOrigin("https://tuck.example.com"), func(c *Config) {
		c.GatePassword = "open sesame"
		c.GateWord = "tuck."
	})
	c := newClient(t, ts)

	s, b := c.call("POST", "/api/gate/passkey/start", nil)
	expect(t, s, 200, b)
	if b["challenge"] == nil || b["challenge"] == "" {
		t.Fatalf("no challenge: %v", b)
	}
	if b["rpId"] != "tuck.example.com" {
		t.Fatalf("rpId %v", b["rpId"])
	}
	if b["userVerification"] != "required" {
		t.Fatalf("the door must verify the user, got %v", b["userVerification"])
	}
	if allowed, ok := b["allowCredentials"].([]any); ok && len(allowed) > 0 {
		t.Fatalf("the door must not name credentials it knows: %v", allowed)
	}
	hints, _ := b["hints"].([]any)
	if len(hints) != 1 || hints[0] != "client-device" {
		t.Fatalf("hints %v, want [client-device]", b["hints"])
	}
}

func TestDoorStaysShutForRubbishPasskeys(t *testing.T) {
	ts, _ := newTestServer(t, withOrigin("https://tuck.example.com"), func(c *Config) {
		c.GatePassword = "open sesame"
		c.GateWord = "tuck."
	})
	c := newClient(t, ts)

	s, b := c.call("POST", "/api/gate/passkey/finish", map[string]any{"id": "nope"})
	expect(t, s, 403, b)

	s, b = c.call("POST", "/api/gate/passkey/start", nil)
	expect(t, s, 200, b)
	s, b = c.call("POST", "/api/gate/passkey/finish", map[string]any{"id": "nope"})
	expect(t, s, 403, b)

	s, b = c.call("GET", "/api/status", nil)
	expect(t, s, 200, b)
	if b["gate"] != true {
		t.Fatalf("the door should still be shut: %v", b)
	}
}

func TestExcludedDevicesAreOnlyEverOfferedAsLocalOnes(t *testing.T) {
	ts, schema := newTestServer(t, withOrigin("https://tuck.example.com"))
	c := newClient(t, ts)
	c.signup(newAccount("alex"))

	_, b := c.call("GET", "/api/passkeys", nil)
	userID := decodeHandle(t, b["userId"].(string))
	dbExec(t, schema, fmt.Sprintf(
		`INSERT INTO %s.passkeys (credential_id, user_id, public_key, aaguid, transports, label)
		 VALUES ($1, $2, $3, $4, $5, $6)`, schema),
		rnd(16), userID, rnd(32), rnd(16), []string{"internal", "hybrid", "usb"}, "this mac")

	s, b := c.call("POST", "/api/passkeys/start", nil)
	expect(t, s, 200, b)
	exclude, _ := b["excludeCredentials"].([]any)
	if len(exclude) != 1 {
		t.Fatalf("the enrolled device should be excluded: %v", b["excludeCredentials"])
	}
	first, _ := exclude[0].(map[string]any)
	transports, _ := first["transports"].([]any)
	if len(transports) != 1 || transports[0] != "internal" {
		t.Fatalf("android refuses the whole ceremony over an off-device transport, got %v", transports)
	}
}

func TestADeviceCanBeGivenABetterName(t *testing.T) {
	ts, schema := newTestServer(t, withOrigin("https://tuck.example.com"))
	c := newClient(t, ts)
	c.signup(newAccount("alex"))

	_, b := c.call("GET", "/api/passkeys", nil)
	userID := decodeHandle(t, b["userId"].(string))
	dbExec(t, schema, fmt.Sprintf(
		`INSERT INTO %s.passkeys (credential_id, user_id, public_key, aaguid, transports, label)
		 VALUES ($1, $2, $3, $4, $5, $6)`, schema),
		rnd(16), userID, rnd(32), rnd(16), []string{"internal"}, "this mac")

	_, b = c.call("GET", "/api/passkeys", nil)
	rows, _ := b["passkeys"].([]any)
	id := rows[0].(map[string]any)["id"].(string)

	s, b := c.call("PATCH", "/api/passkeys/"+id, map[string]any{"label": "  work laptop  "})
	expect(t, s, 200, b)
	rows, _ = b["passkeys"].([]any)
	if rows[0].(map[string]any)["label"] != "work laptop" {
		t.Fatalf("rename did not take: %v", rows[0])
	}

	s, b = c.call("PATCH", "/api/passkeys/"+id, map[string]any{"label": strings.Repeat("x", labelMax+10)})
	expect(t, s, 200, b)
	rows, _ = b["passkeys"].([]any)
	if name, _ := rows[0].(map[string]any)["label"].(string); len([]rune(name)) != labelMax {
		t.Fatalf("a long name should be cut to %d runes, got %d", labelMax, len([]rune(name)))
	}

	s, b = c.call("PATCH", "/api/passkeys/"+id, map[string]any{"label": ""})
	expect(t, s, 200, b)
	rows, _ = b["passkeys"].([]any)
	if rows[0].(map[string]any)["label"] != "this device" {
		t.Fatalf("an empty name falls back: %v", rows[0])
	}

	s, b = c.call("PATCH", "/api/passkeys/not-base64-@@@", map[string]any{"label": "nope"})
	expect(t, s, 400, b)

	s, b = c.call("PATCH", "/api/passkeys/"+base64.RawURLEncoding.EncodeToString(rnd(16)), map[string]any{"label": "nope"})
	expect(t, s, 404, b)
}
