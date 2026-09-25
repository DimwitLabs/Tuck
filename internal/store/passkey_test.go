package store

import (
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// Needs a real Postgres at TUCK_TEST_DATABASE_URL; each test gets its own schema.
func samplePasskey(userID, label string) Passkey {
	return Passkey{
		CredentialID: randomBytes(16),
		UserID:       userID,
		PublicKey:    randomBytes(64),
		AAGUID:       randomBytes(16),
		SignCount:    0,
		BackedUp:     true,
		Transports:   []string{"internal", "hybrid"},
		Label:        label,
	}
}

func TestPasskeysRoundTripThroughTheStore(t *testing.T) {
	st, ctx := newTestStore(t)
	userID := newTestUser(t, st, ctx, "alex")

	first := samplePasskey(userID, "this mac")
	second := samplePasskey(userID, "this iphone")
	for _, p := range []Passkey{first, second} {
		if err := st.AddPasskey(ctx, p); err != nil {
			t.Fatal(err)
		}
	}

	rows, err := st.Passkeys(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 passkeys, got %d", len(rows))
	}
	got := rows[0]
	if got.Label != "this mac" || !got.BackedUp || len(got.Transports) != 2 || got.CreatedAt.IsZero() {
		t.Fatalf("stored passkey came back wrong: %+v", got)
	}
	if got.LastUsedAt != nil {
		t.Fatalf("an unused passkey has no last use: %v", got.LastUsedAt)
	}
	if string(got.PublicKey) != string(first.PublicKey) || string(got.AAGUID) != string(first.AAGUID) {
		t.Fatal("key material did not survive the round trip")
	}
}

func TestPasskeyByIDFindsOneWithoutKnowingTheAccount(t *testing.T) {
	st, ctx := newTestStore(t)
	userID := newTestUser(t, st, ctx, "alex")
	p := samplePasskey(userID, "this mac")
	if err := st.AddPasskey(ctx, p); err != nil {
		t.Fatal(err)
	}

	found, err := st.PasskeyByID(ctx, p.CredentialID)
	if err != nil {
		t.Fatal(err)
	}
	if found.UserID != userID || found.Label != "this mac" {
		t.Fatalf("found the wrong passkey: %+v", found)
	}

	if _, err := st.PasskeyByID(ctx, randomBytes(16)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("an unknown credential should be ErrNotFound, got %v", err)
	}
}

func TestPasskeyUsedRecordsTheCounterAndTheMoment(t *testing.T) {
	st, ctx := newTestStore(t)
	userID := newTestUser(t, st, ctx, "alex")
	p := samplePasskey(userID, "this mac")
	if err := st.AddPasskey(ctx, p); err != nil {
		t.Fatal(err)
	}

	if err := st.PasskeyUsed(ctx, p.CredentialID, 42); err != nil {
		t.Fatal(err)
	}
	found, err := st.PasskeyByID(ctx, p.CredentialID)
	if err != nil {
		t.Fatal(err)
	}
	if found.SignCount != 42 {
		t.Fatalf("sign count %d, want 42", found.SignCount)
	}
	if found.LastUsedAt == nil || found.LastUsedAt.IsZero() {
		t.Fatal("using a passkey should stamp the time")
	}

	if err := st.PasskeyUsed(ctx, randomBytes(16), 1); err != nil {
		t.Fatalf("touching an unknown credential is not an error: %v", err)
	}
}

func TestDeletePasskeyOnlyTouchesTheOwner(t *testing.T) {
	st, ctx := newTestStore(t)
	owner := newTestUser(t, st, ctx, "alex")
	p := samplePasskey(owner, "this mac")
	if err := st.AddPasskey(ctx, p); err != nil {
		t.Fatal(err)
	}

	if err := st.DeletePasskey(ctx, "6f1c2a3b-4d5e-4f60-8a9b-0c1d2e3f4a5b", p.CredentialID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("another account must not be able to forget it: %v", err)
	}
	if err := st.DeletePasskey(ctx, owner, p.CredentialID); err != nil {
		t.Fatal(err)
	}
	if err := st.DeletePasskey(ctx, owner, p.CredentialID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("forgetting twice should be ErrNotFound, got %v", err)
	}
	rows, err := st.Passkeys(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected no passkeys left, got %d", len(rows))
	}
}

func TestPasskeysDieWithTheirAccount(t *testing.T) {
	st, ctx := newTestStore(t)
	userID := newTestUser(t, st, ctx, "alex")
	schema := st.schema
	p := samplePasskey(userID, "this mac")
	if err := st.AddPasskey(ctx, p); err != nil {
		t.Fatal(err)
	}

	conn, err := pgx.Connect(ctx, os.Getenv("TUCK_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err := conn.Exec(ctx, fmt.Sprintf("DELETE FROM %s.users WHERE id = $1", schema), userID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.PasskeyByID(ctx, p.CredentialID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleting the account must cascade to its passkeys, got %v", err)
	}
}

func TestExpiredSessionsArePurged(t *testing.T) {
	st, ctx := newTestStore(t)
	userID := newTestUser(t, st, ctx, "alex")
	live, dead := randomBytes(32), randomBytes(32)
	if err := st.CreateSession(ctx, live, userID, 3, time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateSession(ctx, dead, userID, 3, -time.Minute); err != nil {
		t.Fatal(err)
	}

	if _, err := st.SessionByToken(ctx, dead); !errors.Is(err, ErrNotFound) {
		t.Fatalf("an expired session must not resolve, got %v", err)
	}
	if err := st.PurgeExpiredSessions(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := st.SessionByToken(ctx, live); err != nil {
		t.Fatalf("the live session should have survived the purge: %v", err)
	}
}
