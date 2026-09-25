package keys

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
)

func testKeys(t *testing.T) *Keys {
	t.Helper()
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		t.Fatal(err)
	}
	k, err := New(secret)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func TestASecretShorterThanTheMinimumIsRefused(t *testing.T) {
	for _, n := range []int{0, 1, 16, 31} {
		if _, err := New(make([]byte, n)); err == nil {
			t.Fatalf("a %d byte secret should be refused", n)
		}
	}
	if _, err := New(make([]byte, 32)); err != nil {
		t.Fatalf("32 bytes is the minimum and should be accepted: %v", err)
	}
}

func TestSecretsParseFromHexOrBase64(t *testing.T) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		t.Fatal(err)
	}
	good := map[string]string{
		"hex":            hex.EncodeToString(raw),
		"hex with space": "  " + hex.EncodeToString(raw) + "\n",
		"base64":         base64.StdEncoding.EncodeToString(raw),
		"base64 raw":     base64.RawStdEncoding.EncodeToString(raw),
		"base64 url":     base64.RawURLEncoding.EncodeToString(raw),
	}
	for name, in := range good {
		got, err := ParseSecret(in)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !bytes.Equal(got, raw) {
			t.Fatalf("%s: decoded to the wrong bytes", name)
		}
	}

	short := make([]byte, 16)
	bad := map[string]string{
		"empty":         "",
		"spaces":        "   ",
		"too short hex": hex.EncodeToString(short),
		"too short b64": base64.StdEncoding.EncodeToString(short),
		"not encoded":   "hunter2 but longer than thirty two characters",
	}
	for name, in := range bad {
		if _, err := ParseSecret(in); err == nil {
			t.Fatalf("%s should have been refused", name)
		}
	}
}

func TestEachJobGetsItsOwnKey(t *testing.T) {
	k := testKeys(t)
	cookie := k.Cookie()
	verifier := k.Verifier(cookie)
	if bytes.Equal(cookie, verifier) {
		t.Fatal("the cookie key must not double as the verifier key")
	}
	sealed, err := k.Seal("aad", cookie)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, cookie) {
		t.Fatal("sealing with a key derived from the same secret must not echo the plaintext")
	}
}

func TestSealingRoundTrips(t *testing.T) {
	k := testKeys(t)
	for _, plain := range [][]byte{nil, {}, []byte("x"), bytes.Repeat([]byte("secret"), 200)} {
		sealed, err := k.Seal("tuck:test:v1", plain)
		if err != nil {
			t.Fatal(err)
		}
		opened, err := k.Unseal("tuck:test:v1", sealed)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(opened, plain) && len(plain) > 0 {
			t.Fatalf("round trip changed %d bytes of plaintext", len(plain))
		}
	}
}

func TestSealingIsNotDeterministic(t *testing.T) {
	k := testKeys(t)
	first, err := k.Seal("aad", []byte("same"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := k.Seal("aad", []byte("same"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(first, second) {
		t.Fatal("a repeated nonce would let a dump line up identical values across rows")
	}
}

func TestATamperedSealIsRefused(t *testing.T) {
	k := testKeys(t)
	other := testKeys(t)
	sealed, err := k.Seal("tuck:user:abc:kdf_salt", []byte("the salt"))
	if err != nil {
		t.Fatal(err)
	}

	flip := func(i int) []byte {
		bad := bytes.Clone(sealed)
		bad[i] ^= 1
		return bad
	}
	cases := map[string]struct {
		aad  string
		blob []byte
	}{
		"nonce flipped":      {"tuck:user:abc:kdf_salt", flip(0)},
		"ciphertext flipped": {"tuck:user:abc:kdf_salt", flip(len(sealed) - 1)},
		"tag flipped":        {"tuck:user:abc:kdf_salt", flip(len(sealed) - 3)},
		"truncated":          {"tuck:user:abc:kdf_salt", sealed[:len(sealed)-1]},
		"nonce only":         {"tuck:user:abc:kdf_salt", sealed[:12]},
		"empty":              {"tuck:user:abc:kdf_salt", nil},
		"another user":       {"tuck:user:xyz:kdf_salt", sealed},
		"another field":      {"tuck:user:abc:wrapped_key", sealed},
		"no aad":             {"", sealed},
	}
	for name, c := range cases {
		if _, err := k.Unseal(c.aad, c.blob); !errors.Is(err, ErrTampered) {
			t.Fatalf("%s: want ErrTampered, got %v", name, err)
		}
	}
	if _, err := other.Unseal("tuck:user:abc:kdf_salt", sealed); !errors.Is(err, ErrTampered) {
		t.Fatal("another instance's secret must not open this seal")
	}
}

func TestVerifiersAreKeyedAndStable(t *testing.T) {
	k := testKeys(t)
	proof := []byte("a proof from the browser")
	first, second := k.Verifier(proof), k.Verifier(proof)
	if !bytes.Equal(first, second) {
		t.Fatal("the same proof must always verify to the same bytes")
	}
	if len(first) != 32 {
		t.Fatalf("want a 32 byte verifier, got %d", len(first))
	}
	if bytes.Equal(first, k.Verifier([]byte("a proof from the browse"))) {
		t.Fatal("a different proof must verify differently")
	}
	if bytes.Equal(first, testKeys(t).Verifier(proof)) {
		t.Fatal("without the secret a dump cannot test guesses against the verifier")
	}
	if strings.Contains(string(first), string(proof)) {
		t.Fatal("the verifier must not carry the proof")
	}
}
