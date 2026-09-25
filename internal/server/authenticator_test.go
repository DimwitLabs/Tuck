package server

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"testing"

	"github.com/fxamacker/cbor/v2"
)

// A platform authenticator in twenty lines: enough to prove a passkey enrolled here opens the door here.
type fakeAuthenticator struct {
	key       *ecdsa.PrivateKey
	rpID      string
	origin    string
	credID    []byte
	aaguid    []byte
	signCount uint32
}

func newAuthenticator(t *testing.T, rpID, origin string) *fakeAuthenticator {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return &fakeAuthenticator{key: key, rpID: rpID, origin: origin, credID: rnd(32), aaguid: make([]byte, 16)}
}

func b64u(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func (a *fakeAuthenticator) clientData(t *testing.T, kind, challenge string) []byte {
	t.Helper()
	return []byte(`{"type":"` + kind + `","challenge":"` + challenge + `","origin":"` + a.origin + `","crossOrigin":false}`)
}

func (a *fakeAuthenticator) coseKey(t *testing.T) []byte {
	t.Helper()
	raw, err := cbor.Marshal(map[int]any{
		1:  2,
		3:  -7,
		-1: 1,
		-2: a.key.PublicKey.X.FillBytes(make([]byte, 32)),
		-3: a.key.PublicKey.Y.FillBytes(make([]byte, 32)),
	})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func (a *fakeAuthenticator) authData(t *testing.T, attested bool) []byte {
	t.Helper()
	hash := sha256.Sum256([]byte(a.rpID))
	flags := byte(0x01 | 0x04) // user present, user verified
	data := append([]byte{}, hash[:]...)
	if attested {
		flags |= 0x40
	}
	data = append(data, flags)
	counter := make([]byte, 4)
	binary.BigEndian.PutUint32(counter, a.signCount)
	data = append(data, counter...)
	if attested {
		data = append(data, a.aaguid...)
		length := make([]byte, 2)
		binary.BigEndian.PutUint16(length, uint16(len(a.credID)))
		data = append(data, length...)
		data = append(data, a.credID...)
		data = append(data, a.coseKey(t)...)
	}
	return data
}

// The "none" attestation format: no attestation statement, just the credential.
func (a *fakeAuthenticator) enrol(t *testing.T, challenge string) map[string]any {
	t.Helper()
	clientData := a.clientData(t, "webauthn.create", challenge)
	attestation, err := cbor.Marshal(map[string]any{
		"fmt":      "none",
		"attStmt":  map[string]any{},
		"authData": a.authData(t, true),
	})
	if err != nil {
		t.Fatal(err)
	}
	return map[string]any{
		"id": b64u(a.credID), "rawId": b64u(a.credID), "type": "public-key",
		"clientExtensionResults": map[string]any{},
		"response": map[string]any{
			"clientDataJSON":    b64u(clientData),
			"attestationObject": b64u(attestation),
			"transports":        []string{"internal"},
		},
	}
}

func (a *fakeAuthenticator) assert(t *testing.T, challenge, userHandle string) map[string]any {
	t.Helper()
	a.signCount++
	clientData := a.clientData(t, "webauthn.get", challenge)
	authData := a.authData(t, false)
	hash := sha256.Sum256(clientData)
	digest := sha256.Sum256(append(append([]byte{}, authData...), hash[:]...))
	signature, err := ecdsa.SignASN1(rand.Reader, a.key, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return map[string]any{
		"id": b64u(a.credID), "rawId": b64u(a.credID), "type": "public-key",
		"clientExtensionResults": map[string]any{},
		"response": map[string]any{
			"clientDataJSON":    b64u(clientData),
			"authenticatorData": b64u(authData),
			"signature":         b64u(signature),
			"userHandle":        userHandle,
		},
	}
}

func openDoor(t *testing.T, c *client) {
	t.Helper()
	s, b := c.call("POST", "/api/gate", map[string]any{"typed": "open sesame"})
	expect(t, s, 200, b)
	if b["open"] != true {
		t.Fatalf("the door refused the word: %v", b)
	}
}

func TestAPasskeyEnrolledHereOpensTheDoorHere(t *testing.T) {
	const origin = "https://tuck.example.com"
	ts, _ := newTestServer(t, withOrigin(origin), func(c *Config) {
		c.GatePassword = "open sesame"
		c.GateWord = "tuck."
	})
	c := newClient(t, ts)
	openDoor(t, c)
	c.signup(newAccount("alex"))
	device := newAuthenticator(t, "tuck.example.com", origin)

	_, list := c.call("GET", "/api/passkeys", nil)
	userHandle := list["userId"].(string)

	s, options := c.call("POST", "/api/passkeys/start", nil)
	expect(t, s, 200, options)
	s, b := c.call("POST", "/api/passkeys/finish?label=this%20mac", device.enrol(t, options["challenge"].(string)))
	expect(t, s, 200, b)

	rows, _ := b["passkeys"].([]any)
	if len(rows) != 1 {
		t.Fatalf("enrolment should leave one passkey: %v", b)
	}
	enrolled, _ := rows[0].(map[string]any)
	if enrolled["label"] != "this mac" || enrolled["lastUsed"] != nil {
		t.Fatalf("stored row looks wrong: %v", enrolled)
	}

	// A second enrolment from the same device must be refused by the exclusion list.
	s, options = c.call("POST", "/api/passkeys/start", nil)
	expect(t, s, 200, options)
	if excluded, _ := options["excludeCredentials"].([]any); len(excluded) != 1 {
		t.Fatalf("the enrolled credential should be excluded: %v", options["excludeCredentials"])
	}

	// Now walk up to the door as a stranger would, and let the device answer.
	door := newClient(t, ts)
	_, status := door.call("GET", "/api/status", nil)
	if status["gate"] != true {
		t.Fatalf("the door should be shut for a new visitor: %v", status)
	}

	s, assertion := door.call("POST", "/api/gate/passkey/start", nil)
	expect(t, s, 200, assertion)
	s, b = door.call("POST", "/api/gate/passkey/finish", device.assert(t, assertion["challenge"].(string), userHandle))
	expect(t, s, 200, b)
	if b["open"] != true {
		t.Fatalf("the door should have opened: %v", b)
	}

	_, status = door.call("GET", "/api/status", nil)
	if status["gate"] == true {
		t.Fatalf("the door should be open now: %v", status)
	}

	// And the vault remembers that this passkey was used.
	_, b = c.call("GET", "/api/passkeys", nil)
	rows, _ = b["passkeys"].([]any)
	used, _ := rows[0].(map[string]any)
	if used["lastUsed"] == nil {
		t.Fatalf("opening the door should stamp the passkey: %v", used)
	}
}

func TestADoorPasskeyFromAnotherAccountIsRefused(t *testing.T) {
	const origin = "https://tuck.example.com"
	ts, _ := newTestServer(t, withOrigin(origin), func(c *Config) {
		c.GatePassword = "open sesame"
		c.GateWord = "tuck."
	})
	c := newClient(t, ts)
	openDoor(t, c)
	c.signup(newAccount("alex"))
	device := newAuthenticator(t, "tuck.example.com", origin)

	s, options := c.call("POST", "/api/passkeys/start", nil)
	expect(t, s, 200, options)
	s, b := c.call("POST", "/api/passkeys/finish", device.enrol(t, options["challenge"].(string)))
	expect(t, s, 200, b)

	door := newClient(t, ts)
	s, assertion := door.call("POST", "/api/gate/passkey/start", nil)
	expect(t, s, 200, assertion)
	wrongHandle := b64u([]byte("6f1c2a3b-4d5e-4f60-8a9b-0c1d2e3f4a5b"))
	s, b = door.call("POST", "/api/gate/passkey/finish", device.assert(t, assertion["challenge"].(string), wrongHandle))
	expect(t, s, 403, b)
}
