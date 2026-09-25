package store

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/DimwitLabs/tuck/internal/keys"
)

// rawRow reads a column straight out of postgres, bypassing the store, to see
// what a stolen dump would actually contain.
func rawRow(t *testing.T, ctx context.Context, schema, query string) []byte {
	t.Helper()
	conn, err := pgx.Connect(ctx, os.Getenv("TUCK_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	var out []byte
	if err := conn.QueryRow(ctx, fmt.Sprintf(query, pgx.Identifier{schema}.Sanitize())).Scan(&out); err != nil {
		t.Fatal(err)
	}
	return out
}

func openSealed(t *testing.T, k *keys.Keys, schema string) (*Store, error) {
	t.Helper()
	url := os.Getenv("TUCK_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TUCK_TEST_DATABASE_URL not set")
	}
	return Open(context.Background(), url, schema, k)
}

func dropSchema(t *testing.T, schema string) {
	t.Cleanup(func() {
		ctx := context.Background()
		conn, err := pgx.Connect(ctx, os.Getenv("TUCK_TEST_DATABASE_URL"))
		if err == nil {
			_, _ = conn.Exec(ctx, "DROP SCHEMA IF EXISTS "+pgx.Identifier{schema}.Sanitize()+" CASCADE")
			conn.Close(ctx)
		}
	})
}

func TestTheDatabaseNeverHoldsTheDerivationInputs(t *testing.T) {
	st, ctx := newTestStore(t)
	var steps [3]Step
	plainSalts := [3][]byte{}
	for i := range steps {
		plainSalts[i] = randomBytes(16)
		steps[i] = Step{Salt: plainSalts[i], QuestionNonce: randomBytes(12), QuestionCiphertext: randomBytes(40), Proof: randomBytes(32)}
	}
	kdfSalt, authKey, wrapped := randomBytes(16), randomBytes(32), randomBytes(48)
	err := st.CreateUser(ctx, testUserID, "havi", Enrollment{
		KDF:     KDF{Algorithm: "argon2id", Iterations: 3, MemoryKiB: 65536, Parallelism: 1},
		KDFSalt: kdfSalt, AuthKey: authKey, Steps: steps,
		WrappedKey: wrapped, WrappedNonce: randomBytes(12),
	})
	if err != nil {
		t.Fatal(err)
	}

	schema := st.schema[1 : len(st.schema)-1]
	stored := map[string][]byte{
		"kdf_salt":            rawRow(t, ctx, schema, "SELECT kdf_salt FROM %s.users"),
		"wrapped_key":         rawRow(t, ctx, schema, "SELECT wrapped_key FROM %s.users"),
		"auth_hash":           rawRow(t, ctx, schema, "SELECT auth_hash FROM %s.users"),
		"salt":                rawRow(t, ctx, schema, "SELECT salt FROM %s.unlock_steps WHERE step = 1"),
		"question_ciphertext": rawRow(t, ctx, schema, "SELECT question_ciphertext FROM %s.unlock_steps WHERE step = 1"),
		"proof_hash":          rawRow(t, ctx, schema, "SELECT proof_hash FROM %s.unlock_steps WHERE step = 1"),
	}
	plain := map[string][]byte{
		"kdf_salt":            kdfSalt,
		"wrapped_key":         wrapped,
		"auth_hash":           authKey,
		"salt":                plainSalts[0],
		"question_ciphertext": steps[0].QuestionCiphertext,
		"proof_hash":          steps[0].Proof,
	}
	for column, raw := range stored {
		if bytes.Equal(raw, plain[column]) {
			t.Fatalf("%s is stored verbatim; a dump would hand over the real value", column)
		}
		if bytes.Contains(raw, plain[column]) {
			t.Fatalf("%s carries its plaintext inside the stored bytes", column)
		}
	}
	if len(stored["kdf_salt"]) != len(kdfSalt)+28 {
		t.Fatalf("a sealed salt should carry a 12 byte nonce and a 16 byte tag, got %d bytes for %d", len(stored["kdf_salt"]), len(kdfSalt))
	}

	u, err := st.UserByName(ctx, "havi")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(u.KDFSalt, kdfSalt) || !bytes.Equal(u.WrappedKey, wrapped) {
		t.Fatal("reading a user back must unseal to exactly what was enrolled")
	}
	step, err := st.Step(ctx, testUserID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(step.Salt, plainSalts[0]) || !bytes.Equal(step.QuestionCiphertext, steps[0].QuestionCiphertext) {
		t.Fatal("reading a step back must unseal to exactly what was enrolled")
	}
}

func TestAuthMatchesOnlyTheEnrolledKey(t *testing.T) {
	st, ctx := newTestStore(t)
	authKey := randomBytes(32)
	var steps [3]Step
	for i := range steps {
		steps[i] = Step{Salt: randomBytes(16), QuestionNonce: randomBytes(12), QuestionCiphertext: randomBytes(40), Proof: randomBytes(32)}
	}
	if err := st.CreateUser(ctx, testUserID, "havi", Enrollment{
		KDF:     KDF{Algorithm: "argon2id", Iterations: 3, MemoryKiB: 65536, Parallelism: 1},
		KDFSalt: randomBytes(16), AuthKey: authKey, Steps: steps,
		WrappedKey: randomBytes(48), WrappedNonce: randomBytes(12),
	}); err != nil {
		t.Fatal(err)
	}
	u, err := st.UserByName(ctx, "havi")
	if err != nil {
		t.Fatal(err)
	}
	if !st.AuthMatches(u, authKey) {
		t.Fatal("the enrolled auth key must match")
	}
	if st.AuthMatches(u, randomBytes(32)) {
		t.Fatal("any other auth key must not match")
	}
}

func TestAnUnknownNameCostsWhatAKnownOneCosts(t *testing.T) {
	st, ctx := newTestStore(t)
	newTestUser(t, st, ctx, "havi")

	real, found, err := st.UserByNameOrDecoy(ctx, "havi")
	if err != nil || !found {
		t.Fatalf("known name: found=%v err=%v", found, err)
	}
	decoy, found, err := st.UserByNameOrDecoy(ctx, "nobody")
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Fatal("an unknown name must not report as found")
	}
	// The decoy came back unsealed, which is the work a real lookup pays for.
	if len(decoy.KDFSalt) != len(real.KDFSalt) || len(decoy.WrappedKey) != len(real.WrappedKey) {
		t.Fatalf("the decoy must be unsealed like a real user: salt %d vs %d, key %d vs %d",
			len(decoy.KDFSalt), len(real.KDFSalt), len(decoy.WrappedKey), len(real.WrappedKey))
	}
	if decoy.KDF != real.KDF {
		t.Fatal("the decoy must advertise the same kdf settings a real user would")
	}
	if st.AuthMatches(decoy, randomBytes(32)) {
		t.Fatal("nothing should authenticate against the decoy")
	}
}

func TestAnotherSecretCannotOpenTheDatabase(t *testing.T) {
	first, err := keys.New(bytes.Repeat([]byte{1}, 32))
	if err != nil {
		t.Fatal(err)
	}
	second, err := keys.New(bytes.Repeat([]byte{2}, 32))
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("tuck_test_%x", randomBytes(6))
	dropSchema(t, schema)

	st, err := openSealed(t, first, schema)
	if err != nil {
		t.Fatal(err)
	}
	st.Close()

	if _, err := openSealed(t, second, schema); !errors.Is(err, ErrWrongSecret) {
		t.Fatalf("starting with the wrong secret must fail loudly, got %v", err)
	}
	again, err := openSealed(t, first, schema)
	if err != nil {
		t.Fatalf("the original secret must still open it: %v", err)
	}
	again.Close()
}

func TestUnfreezeClearsTheFlag(t *testing.T) {
	st, ctx := newTestStore(t)
	newTestUser(t, st, ctx, "havi")
	lockout := Lockout{Every: 1, Base: time.Hour, FreezeAfter: 0}
	if _, err := st.CheckAnswer(ctx, testUserID, 1, randomBytes(32), lockout); err != nil {
		t.Fatal(err)
	}
	u, err := st.UserByName(ctx, "havi")
	if err != nil {
		t.Fatal(err)
	}
	if !u.UnlockFrozen {
		t.Fatal("a wrong answer past the freeze threshold should freeze the account")
	}
	if err := st.Unfreeze(ctx, "havi"); err != nil {
		t.Fatal(err)
	}
	if u, err = st.UserByName(ctx, "havi"); err != nil || u.UnlockFrozen {
		t.Fatalf("unfreeze should clear the flag: frozen=%v err=%v", u.UnlockFrozen, err)
	}
	if err := st.Unfreeze(ctx, "nobody"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unfreezing an unknown user should say so, got %v", err)
	}
}

func TestARekeyResealsEverything(t *testing.T) {
	st, ctx := newTestStore(t)
	newTestUser(t, st, ctx, "havi")
	before, err := st.UserByName(ctx, "havi")
	if err != nil {
		t.Fatal(err)
	}

	var steps [3]Step
	for i := range steps {
		steps[i] = Step{Salt: randomBytes(16), QuestionNonce: randomBytes(12), QuestionCiphertext: randomBytes(40), Proof: randomBytes(32)}
	}
	fresh := Enrollment{
		KDF:     KDF{Algorithm: "argon2id", Iterations: 3, MemoryKiB: 65536, Parallelism: 1},
		KDFSalt: randomBytes(16), AuthKey: randomBytes(32), Steps: steps,
		WrappedKey: randomBytes(48), WrappedNonce: randomBytes(12),
	}
	if err := st.Rekey(ctx, testUserID, randomBytes(32), fresh); err != nil {
		t.Fatal(err)
	}

	after, err := st.UserByName(ctx, "havi")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after.KDFSalt, fresh.KDFSalt) || !bytes.Equal(after.WrappedKey, fresh.WrappedKey) {
		t.Fatal("a rekey must store and unseal the new enrollment")
	}
	if bytes.Equal(after.KDFSalt, before.KDFSalt) {
		t.Fatal("the old salt should be gone")
	}
	if !st.AuthMatches(after, fresh.AuthKey) {
		t.Fatal("the new auth key must match after a rekey")
	}
	step, err := st.Step(ctx, testUserID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(step.Salt, steps[1].Salt) {
		t.Fatal("a rekey must reseal every step, not just the user row")
	}
}

func TestATamperedRowIsRefused(t *testing.T) {
	st, ctx := newTestStore(t)
	newTestUser(t, st, ctx, "havi")
	schema := st.schema[1 : len(st.schema)-1]

	exec := func(sql string) {
		t.Helper()
		conn, err := pgx.Connect(ctx, os.Getenv("TUCK_TEST_DATABASE_URL"))
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close(ctx)
		if _, err := conn.Exec(ctx, fmt.Sprintf(sql, pgx.Identifier{schema}.Sanitize())); err != nil {
			t.Fatal(err)
		}
	}

	// Flip one byte of the sealed salt, the way a database-side edit would.
	exec("UPDATE %s.users SET kdf_salt = set_byte(kdf_salt, 20, get_byte(kdf_salt, 20) # 1)")
	if _, err := st.UserByName(ctx, "havi"); !errors.Is(err, keys.ErrTampered) {
		t.Fatalf("an edited user row must be refused, got %v", err)
	}
	exec("UPDATE %s.users SET kdf_salt = set_byte(kdf_salt, 20, get_byte(kdf_salt, 20) # 1)")
	if _, err := st.UserByName(ctx, "havi"); err != nil {
		t.Fatalf("putting the byte back should read again: %v", err)
	}

	exec("UPDATE %s.unlock_steps SET question_ciphertext = set_byte(question_ciphertext, 20, get_byte(question_ciphertext, 20) # 1) WHERE step = 2")
	if _, err := st.Step(ctx, testUserID, 2); !errors.Is(err, keys.ErrTampered) {
		t.Fatalf("an edited question must be refused, got %v", err)
	}

	// A step's seal is bound to its own number, so moving one is not a way in.
	exec("UPDATE %s.unlock_steps SET salt = (SELECT salt FROM %[1]s.unlock_steps WHERE step = 1) WHERE step = 3")
	if _, err := st.Step(ctx, testUserID, 3); !errors.Is(err, keys.ErrTampered) {
		t.Fatalf("a salt moved between steps must be refused, got %v", err)
	}
}

func TestAMissingStepIsNotFound(t *testing.T) {
	st, ctx := newTestStore(t)
	newTestUser(t, st, ctx, "havi")
	if _, err := st.Step(ctx, testUserID, 9); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound for a step that was never enrolled, got %v", err)
	}
}

func TestAnUnreachableDatabaseIsReported(t *testing.T) {
	k, err := keys.New(bytes.Repeat([]byte{3}, 32))
	if err != nil {
		t.Fatal(err)
	}
	if st, err := Open(context.Background(), "postgres://nobody@127.0.0.1:1/nothing", "tuck_test_none", k); err == nil {
		st.Close()
		t.Fatal("opening a database that is not there must fail rather than start half-built")
	}
}

func TestAMissingUserIsNotFound(t *testing.T) {
	st, ctx := newTestStore(t)
	if _, err := st.UserByID(ctx, "00000000-0000-4000-8000-000000000000"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound for an id nobody has, got %v", err)
	}
}
