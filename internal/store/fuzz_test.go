package store

import "testing"

func FuzzCheckTransport(f *testing.F) {
	for _, s := range []string{
		"", "postgres://tuck:pw@localhost/tuck", "postgres://tuck:pw@db.example.com/tuck",
		"postgres://tuck:pw@db.example.com/tuck?sslmode=require", "postgres://tuck:pw@/tuck?host=/var/run/postgresql",
		"host=127.0.0.1 sslmode=disable", "://", "postgres://[::1]/tuck",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, url string) {
		_ = CheckTransport(url)
	})
}
