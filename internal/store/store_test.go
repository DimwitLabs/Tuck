package store

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/DimwitLabs/tuck/internal/keys"
)

func newTestStore(t *testing.T) (*Store, context.Context) {
	t.Helper()
	url := os.Getenv("TUCK_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TUCK_TEST_DATABASE_URL not set")
	}
	schema := fmt.Sprintf("tuck_test_%x", randomBytes(6))
	ctx := context.Background()
	k, err := keys.New(randomBytes(32))
	if err != nil {
		t.Fatal(err)
	}
	st, err := Open(ctx, url, schema, k)
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
	return st, ctx
}

func randomBytes(n int) []byte {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return b
}

const testUserID = "3f1c2a3b-4d5e-4f60-8a9b-0c1d2e3f4a5b"

func newTestUser(t *testing.T, st *Store, ctx context.Context, name string) string {
	t.Helper()
	var steps [3]Step
	for i := range steps {
		steps[i] = Step{Salt: randomBytes(16), QuestionNonce: randomBytes(12), QuestionCiphertext: randomBytes(40), Proof: randomBytes(32)}
	}
	err := st.CreateUser(ctx, testUserID, name, Enrollment{
		KDF:     KDF{Algorithm: "argon2id", Iterations: 3, MemoryKiB: 65536, Parallelism: 1},
		KDFSalt: randomBytes(16), AuthKey: randomBytes(32), Steps: steps,
		WrappedKey: randomBytes(48), WrappedNonce: randomBytes(12),
	})
	if err != nil {
		t.Fatal(err)
	}
	return testUserID
}
