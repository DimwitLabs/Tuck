package store

import "testing"

func TestCheckTransport(t *testing.T) {
	for url, ok := range map[string]bool{
		"postgres://tuck:x@db:5432/tuck?sslmode=disable":                            false,
		"postgres://tuck:x@localhost:5432/tuck?sslmode=disable":                     true,
		"postgres://tuck:x@127.0.0.1:5432/tuck":                                     true,
		"host=/var/run/postgresql dbname=tuck":                                      true,
		"postgres://u:x@aws-0-eu.pooler.supabase.com:6543/postgres?sslmode=require": true,
		"postgres://u:x@db.example.com/tuck?sslmode=verify-full":                    true,
		"postgres://u:x@db.example.com/tuck":                                        false,
		"postgres://u:x@db.example.com/tuck?sslmode=prefer":                         false,
		"postgres://u:x@db.example.com/tuck?sslmode=disable":                        false,
		"postgres://u:x@10.0.0.5/tuck?sslmode=disable":                              false,
	} {
		if err := CheckTransport(url); (err == nil) != ok {
			t.Errorf("%s: got %v, want ok=%v", url, err, ok)
		}
	}
}
