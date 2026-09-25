package server

import (
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
)

func FuzzParseProxies(f *testing.F) {
	for _, s := range []string{"", "10.0.0.1", "172.16.0.0/12, ::1", "fe80::/10", "not an address", "2001:db8::1/129", "::ffff:10.0.0.1"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, list string) {
		out, err := ParseProxies(list)
		if err != nil {
			return
		}
		for _, p := range out {
			if !p.IsValid() || p != p.Masked() {
				t.Fatalf("%q gave an invalid or unmasked prefix %v", list, p)
			}
		}
	})
}

func FuzzClientIP(f *testing.F) {
	f.Add("203.0.113.7:4321", "")
	f.Add("10.0.0.1:1", "198.51.100.9, 10.0.0.2")
	f.Add("[2001:db8::1]:80", "garbage,, 10.0.0.1")
	f.Add("10.0.0.1:1", "10.0.0.3, 10.0.0.2")
	f.Add("nonsense", "1.2.3.4")
	proxies, err := ParseProxies("10.0.0.0/8, 2001:db8::/32")
	if err != nil {
		f.Fatal(err)
	}
	s := &Server{cfg: Config{TrustedProxies: proxies}}
	f.Fuzz(func(t *testing.T, remote, forwarded string) {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = remote
		r.Header.Set("X-Forwarded-For", forwarded)
		ip := s.clientIP(r)
		if !s.fromProxy(r) && ip != peer(r) {
			t.Fatalf("an untrusted peer %q must be reported as itself, got %q", remote, ip)
		}
		if s.fromProxy(r) && s.trusted(ip) {
			hops := strings.Split(forwarded, ",")
			for i := len(hops) - 1; i >= 0; i-- {
				hop := strings.TrimSpace(hops[i])
				if hop == "" {
					break
				}
				if !s.trusted(hop) {
					t.Fatalf("%q behind %q reported the proxy %q instead of the client", forwarded, remote, ip)
				}
			}
		}
		_ = limiterKey(ip)
	})
}

func FuzzLimiterKey(f *testing.F) {
	for _, s := range []string{"", "1.2.3.4", "2001:db8::1", "::ffff:1.2.3.4", "fe80::1%eth0", "junk"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, ip string) {
		key := limiterKey(ip)
		a, err := netip.ParseAddr(ip)
		if err != nil || a.Unmap().Is4() {
			if key != ip {
				t.Fatalf("%q should be its own key, got %q", ip, key)
			}
			return
		}
		p, err := netip.ParsePrefix(key)
		if err != nil || p.Bits() != 64 || !p.Contains(a.WithZone("")) {
			t.Fatalf("%q should map to its /64, got %q", ip, key)
		}
	})
}

func FuzzEnrollmentParse(f *testing.F) {
	zeros := func(n int) string { return base64.StdEncoding.EncodeToString(make([]byte, n)) }
	step := stepJSON{Salt: zeros(16), QuestionNonce: zeros(12), QuestionCiphertext: zeros(40), Proof: zeros(32)}
	valid, _ := json.Marshal(enrollmentJSON{
		KDF: defaultKDF, KDFSalt: zeros(16), AuthKey: zeros(32), Steps: []stepJSON{step, step, step},
		WrappedKey: zeros(48), WrappedNonce: zeros(12),
	})
	f.Add(valid)
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"kdf":{"algorithm":"argon2id","iterations":1,"memoryKiB":1,"parallelism":1}}`))
	f.Add([]byte(`not json`))
	f.Fuzz(func(t *testing.T, data []byte) {
		var e enrollmentJSON
		if json.Unmarshal(data, &e) != nil {
			return
		}
		out, err := e.parse()
		if err != nil {
			return
		}
		if len(out.KDFSalt) != 16 || len(out.AuthKey) != 32 || len(out.WrappedKey) != 48 || len(out.WrappedNonce) != 12 {
			t.Fatalf("accepted an enrollment with the wrong sizes: %+v", out)
		}
		for _, st := range out.Steps {
			if len(st.Salt) != 16 || len(st.QuestionNonce) != 12 || len(st.Proof) != 32 ||
				len(st.QuestionCiphertext) < 17 || len(st.QuestionCiphertext) > 1024 {
				t.Fatalf("accepted a step with the wrong sizes: %+v", st)
			}
		}
		if out.KDF.MemoryKiB < 19456 || out.KDF.Iterations < 2 {
			t.Fatalf("accepted weak key derivation settings: %+v", out.KDF)
		}
	})
}

func FuzzSignedValuesCannotBeForged(f *testing.F) {
	s := &Server{cfg: Config{Secret: []byte("fuzz-secret-fuzz-secret-fuzz-secret")}}
	f.Add("gate:x", "9999999999."+strings.Repeat("00", 32))
	f.Add("device:u", "9999999999.00")
	f.Add("", "")
	f.Add("gate:x", ".")
	f.Fuzz(func(t *testing.T, purpose, value string) {
		if s.verify(purpose, value) {
			t.Fatalf("a made-up value %q verified for %q", value, purpose)
		}
	})
}
