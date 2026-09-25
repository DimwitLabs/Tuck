package keys

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

const MinSecretBytes = 32

var ErrTampered = errors.New("sealed value failed authentication")

// Keys splits TUCK_SECRET into one key per job. The secret lives outside the
// database, so a stolen dump carries no way to open what these sealed.
type Keys struct {
	cookie []byte
	verify []byte
	aead   cipher.AEAD
}

func New(secret []byte) (*Keys, error) {
	if len(secret) < MinSecretBytes {
		return nil, fmt.Errorf("secret must be at least %d bytes, got %d", MinSecretBytes, len(secret))
	}
	cookie, err := hkdf.Key(sha256.New, secret, nil, "tuck:cookie:v1", 32)
	if err != nil {
		return nil, err
	}
	verify, err := hkdf.Key(sha256.New, secret, nil, "tuck:verify:v1", 32)
	if err != nil {
		return nil, err
	}
	sealing, err := hkdf.Key(sha256.New, secret, nil, "tuck:seal:v1", 32)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(sealing)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Keys{cookie: cookie, verify: verify, aead: aead}, nil
}

// ParseSecret reads TUCK_SECRET as hex or base64, whichever it parses as.
func ParseSecret(raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("empty")
	}
	if b, err := hex.DecodeString(raw); err == nil && len(b) >= MinSecretBytes {
		return b, nil
	}
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		if b, err := enc.DecodeString(raw); err == nil && len(b) >= MinSecretBytes {
			return b, nil
		}
	}
	return nil, fmt.Errorf("must decode from hex or base64 to at least %d bytes", MinSecretBytes)
}

func (k *Keys) Cookie() []byte { return k.cookie }

// Verifier turns a client-supplied proof into what the database stores. Keyed,
// so a dump alone cannot be used to test guesses against it.
func (k *Keys) Verifier(input []byte) []byte {
	mac := hmac.New(sha256.New, k.verify)
	mac.Write(input)
	return mac.Sum(nil)
}

func (k *Keys) Seal(aad string, plaintext []byte) ([]byte, error) {
	nonce := make([]byte, k.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return k.aead.Seal(nonce, nonce, plaintext, []byte(aad)), nil
}

func (k *Keys) Unseal(aad string, blob []byte) ([]byte, error) {
	if len(blob) < k.aead.NonceSize() {
		return nil, ErrTampered
	}
	nonce, body := blob[:k.aead.NonceSize()], blob[k.aead.NonceSize():]
	out, err := k.aead.Open(nil, nonce, body, []byte(aad))
	if err != nil {
		return nil, ErrTampered
	}
	return out, nil
}
