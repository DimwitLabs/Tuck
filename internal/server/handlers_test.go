package server

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestJanitorStopsWithItsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan struct{})
	go func() {
		(&Server{}).Janitor(ctx)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("a cancelled context should stop the janitor")
	}
}

func TestApiErrorCarriesItsMessage(t *testing.T) {
	err := fail(409, "too many %s", "devices")
	if err.Error() != "too many devices" {
		t.Fatalf("message %q", err.Error())
	}
	if e, ok := err.(*apiError); !ok || e.status != 409 {
		t.Fatalf("status did not survive: %v", err)
	}
}

func TestItemsCanBeDeleted(t *testing.T) {
	ts, _ := newTestServer(t)
	c := newClient(t, ts)
	c.signup(newAccount("alex"))

	s, b := c.call("PUT", "/api/items/"+itemA, item("host"))
	expect(t, s, 204, b)

	s, b = c.call("GET", "/api/items", nil)
	expect(t, s, 200, b)
	if items, _ := b["items"].([]any); len(items) != 1 {
		t.Fatalf("expected the item to be stored: %v", b)
	}

	s, b = c.call("DELETE", "/api/items/"+itemA, nil)
	expect(t, s, 204, b)

	s, b = c.call("GET", "/api/items", nil)
	expect(t, s, 200, b)
	if items, _ := b["items"].([]any); len(items) != 0 {
		t.Fatalf("expected no items left: %v", b)
	}

	s, b = c.call("DELETE", "/api/items/not-a-uuid", nil)
	expect(t, s, 400, b)
}

// The reviewer's check: a name nobody has must cost what a real one costs, and
// say the same thing back.
func TestAnUnknownNameLooksExactlyLikeAKnownOne(t *testing.T) {
	ts, _ := newTestServer(t)
	c := newClient(t, ts)
	a := newAccount("havi")
	c.signup(a)

	s, known := c.call("GET", "/api/prelogin?username=havi", nil)
	expect(t, s, 200, known)
	s, unknown := c.call("GET", "/api/prelogin?username=nobody", nil)
	expect(t, s, 200, unknown)

	for _, field := range []string{"kdf", "kdfSalt"} {
		if unknown[field] == nil {
			t.Fatalf("an unknown name must still answer with %s", field)
		}
	}
	if fmt.Sprint(unknown["kdf"]) != fmt.Sprint(known["kdf"]) {
		t.Fatal("the decoy must advertise the same kdf settings a real account does")
	}
	if unknown["kdfSalt"] == known["kdfSalt"] {
		t.Fatal("the decoy salt must not be the real one")
	}

	_, again := c.call("GET", "/api/prelogin?username=nobody", nil)
	if again["kdfSalt"] != unknown["kdfSalt"] {
		t.Fatal("the decoy salt must be stable per name, or a miss is spottable by asking twice")
	}
	_, other := c.call("GET", "/api/prelogin?username=someoneelse", nil)
	if other["kdfSalt"] == unknown["kdfSalt"] {
		t.Fatal("two unknown names sharing a salt would give the decoy away")
	}

	sWrong, wrong := c.call("POST", "/api/login", map[string]any{"username": "havi", "authKey": b64s(rnd(32))})
	sMissing, missing := c.call("POST", "/api/login", map[string]any{"username": "nobody", "authKey": b64s(rnd(32))})
	if sWrong != sMissing || sWrong != 401 {
		t.Fatalf("a wrong password and an unknown name must both be 401, got %d and %d", sWrong, sMissing)
	}
	if fmt.Sprint(wrong["error"]) != fmt.Sprint(missing["error"]) {
		t.Fatalf("the two must say the same thing: %q vs %q", wrong["error"], missing["error"])
	}
}

func TestBase64FieldsAreCheckedBeforeUse(t *testing.T) {
	good := base64.StdEncoding.EncodeToString(rnd(32))
	cases := map[string]struct {
		value string
		size  int
		want  string
	}{
		"right size":   {good, 32, ""},
		"any size":     {good, 0, ""},
		"wrong size":   {good, 16, "authKey must be 16 bytes"},
		"not base64":   {"not base64!", 32, "authKey is not valid base64"},
		"empty":        {"", 32, "authKey is not valid base64"},
		"decodes to 0": {base64.StdEncoding.EncodeToString(nil), 32, "authKey is not valid base64"},
	}
	for name, c := range cases {
		out, err := b64("authKey", c.value, c.size)
		if c.want == "" {
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if len(out) != 32 {
				t.Fatalf("%s: decoded %d bytes, want 32", name, len(out))
			}
			continue
		}
		if err == nil {
			t.Fatalf("%s: should have been refused", name)
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Fatalf("%s: want %q, got %q", name, c.want, err.Error())
		}
	}
}
