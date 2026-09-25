package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/DimwitLabs/tuck/internal/keys"
)

var ErrNotFound = errors.New("not found")

type Store struct {
	pool   *pgxpool.Pool
	schema string
	keys   *keys.Keys
	decoy  *User
}

// Every query names the schema explicitly: transaction poolers such as Supabase's drop search_path.
func Open(ctx context.Context, databaseURL, schema string, k *keys.Keys) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	s := &Store{pool: pool, schema: pgx.Identifier{schema}.Sanitize(), keys: k}
	if err := s.migrate(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	if err := s.buildDecoy(); err != nil {
		pool.Close()
		return nil, err
	}
	if err := s.checkCanary(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close()                         { s.pool.Close() }
func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

func (s *Store) q(sql string) string { return strings.ReplaceAll(sql, "{s}", s.schema) }

func (s *Store) migrate(ctx context.Context) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, "tuck-migrate:"+s.schema); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, s.q(`CREATE SCHEMA IF NOT EXISTS {s}`)); err != nil {
		return fmt.Errorf("create schema: %w", err)
	}
	if _, err := tx.Exec(ctx, s.q(`CREATE TABLE IF NOT EXISTS {s}.schema_migrations (
		version integer PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())`)); err != nil {
		return err
	}
	var applied int
	if err := tx.QueryRow(ctx, s.q(`SELECT coalesce(max(version), 0) FROM {s}.schema_migrations`)).Scan(&applied); err != nil {
		return err
	}
	for v := applied + 1; v <= len(migrations); v++ {
		if _, err := tx.Exec(ctx, s.q(migrations[v-1])); err != nil {
			return fmt.Errorf("migration %d: %w", v, err)
		}
		if _, err := tx.Exec(ctx, s.q(`INSERT INTO {s}.schema_migrations (version) VALUES ($1)`), v); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

var ErrWrongSecret = errors.New("TUCK_SECRET does not match this database")

const canaryPlaintext = "tuck-canary-v1"

// A wrong or rotated secret should stop the server here, not surface later as
// unlock failures nobody can explain.
func (s *Store) checkCanary(ctx context.Context) error {
	fresh, err := s.keys.Seal("tuck:canary:v1", []byte(canaryPlaintext))
	if err != nil {
		return err
	}
	var stored []byte
	if err := s.pool.QueryRow(ctx, s.q(`
		INSERT INTO {s}.settings (key, value) VALUES ($1, $2)
		ON CONFLICT (key) DO UPDATE SET key = EXCLUDED.key
		RETURNING value`), "canary", fresh).Scan(&stored); err != nil {
		return err
	}
	opened, err := s.keys.Unseal("tuck:canary:v1", stored)
	if err != nil || string(opened) != canaryPlaintext {
		return ErrWrongSecret
	}
	return nil
}

func (s *Store) buildDecoy() error {
	blank := make([]byte, 16)
	if _, err := rand.Read(blank); err != nil {
		return err
	}
	u := &User{ID: "00000000-0000-0000-0000-000000000000", KDF: KDF{}}
	var err error
	if u.KDFSalt, err = s.keys.Seal(userAAD(u.ID, "kdf_salt"), blank); err != nil {
		return err
	}
	if u.WrappedKey, err = s.keys.Seal(userAAD(u.ID, "wrapped_key"), make([]byte, 48)); err != nil {
		return err
	}
	if u.WrappedNonce, err = s.keys.Seal(userAAD(u.ID, "wrapped_nonce"), make([]byte, 12)); err != nil {
		return err
	}
	u.AuthHash = s.keys.Verifier([]byte("tuck-decoy-auth"))
	s.decoy = u
	return nil
}

func userAAD(userID, field string) string { return "tuck:user:" + userID + ":" + field }

func stepAAD(userID string, step int, field string) string {
	return "tuck:step:" + userID + ":" + strconv.Itoa(step) + ":" + field
}

type KDF struct {
	Algorithm   string `json:"algorithm"`
	Iterations  int    `json:"iterations"`
	MemoryKiB   int    `json:"memoryKiB"`
	Parallelism int    `json:"parallelism"`
}

type Step struct {
	Salt               []byte
	QuestionNonce      []byte
	QuestionCiphertext []byte
	Proof              []byte
}

type User struct {
	ID                string
	Username          string
	KDF               KDF
	KDFSalt           []byte
	AuthHash          []byte
	WrappedKey        []byte
	WrappedNonce      []byte
	UnlockLockedUntil *time.Time
	UnlockFrozen      bool
}

type Enrollment struct {
	KDF          KDF
	KDFSalt      []byte
	AuthKey      []byte
	WrappedKey   []byte
	WrappedNonce []byte
	Steps        [3]Step
}

// Everything the browser needs to rebuild the key chain goes in sealed: salts,
// questions and the wrapped vault key. Without them a dump cannot even run the
// KDF, let alone test a guess against it.
func (s *Store) sealEnrollment(userID string, e Enrollment) (Enrollment, error) {
	var err error
	if e.KDFSalt, err = s.keys.Seal(userAAD(userID, "kdf_salt"), e.KDFSalt); err != nil {
		return e, err
	}
	if e.WrappedKey, err = s.keys.Seal(userAAD(userID, "wrapped_key"), e.WrappedKey); err != nil {
		return e, err
	}
	if e.WrappedNonce, err = s.keys.Seal(userAAD(userID, "wrapped_nonce"), e.WrappedNonce); err != nil {
		return e, err
	}
	e.AuthKey = s.keys.Verifier(e.AuthKey)
	for i := range e.Steps {
		step := e.Steps[i]
		if step.Salt, err = s.keys.Seal(stepAAD(userID, i+1, "salt"), step.Salt); err != nil {
			return e, err
		}
		if step.QuestionNonce, err = s.keys.Seal(stepAAD(userID, i+1, "question_nonce"), step.QuestionNonce); err != nil {
			return e, err
		}
		if step.QuestionCiphertext, err = s.keys.Seal(stepAAD(userID, i+1, "question_ciphertext"), step.QuestionCiphertext); err != nil {
			return e, err
		}
		step.Proof = s.keys.Verifier(step.Proof)
		e.Steps[i] = step
	}
	return e, nil
}

func (s *Store) openUser(u *User) error {
	var err error
	if u.KDFSalt, err = s.keys.Unseal(userAAD(u.ID, "kdf_salt"), u.KDFSalt); err != nil {
		return err
	}
	if u.WrappedKey, err = s.keys.Unseal(userAAD(u.ID, "wrapped_key"), u.WrappedKey); err != nil {
		return err
	}
	u.WrappedNonce, err = s.keys.Unseal(userAAD(u.ID, "wrapped_nonce"), u.WrappedNonce)
	return err
}

// AuthMatches compares a login's auth key against the stored verifier.
func (s *Store) AuthMatches(u *User, authKey []byte) bool {
	return subtle.ConstantTimeCompare(s.keys.Verifier(authKey), u.AuthHash) == 1
}

func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, s.q(`SELECT count(*) FROM {s}.users`)).Scan(&n)
	return n, err
}

var ErrSignupClosed = errors.New("signup closed")

func (s *Store) CreateUser(ctx context.Context, id, username string, e Enrollment) error {
	kdf, err := json.Marshal(e.KDF)
	if err != nil {
		return err
	}
	e, err = s.sealEnrollment(id, e)
	if err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, s.q(`LOCK TABLE {s}.users IN EXCLUSIVE MODE`)); err != nil {
			return err
		}
		var exists bool
		if err := tx.QueryRow(ctx, s.q(`SELECT EXISTS (SELECT 1 FROM {s}.users)`)).Scan(&exists); err != nil {
			return err
		}
		if exists {
			return ErrSignupClosed
		}
		if _, err := tx.Exec(ctx, s.q(`
			INSERT INTO {s}.users (id, username, kdf, kdf_salt, auth_hash, wrapped_key, wrapped_nonce)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`),
			id, username, kdf, e.KDFSalt, e.AuthKey, e.WrappedKey, e.WrappedNonce); err != nil {
			return err
		}
		return s.writeSteps(ctx, tx, id, e.Steps)
	})
}

func (s *Store) writeSteps(ctx context.Context, tx pgx.Tx, userID string, steps [3]Step) error {
	if _, err := tx.Exec(ctx, s.q(`DELETE FROM {s}.unlock_steps WHERE user_id = $1`), userID); err != nil {
		return err
	}
	for i, st := range steps {
		if _, err := tx.Exec(ctx, s.q(`
			INSERT INTO {s}.unlock_steps (user_id, step, salt, question_nonce, question_ciphertext, proof_hash)
			VALUES ($1, $2, $3, $4, $5, $6)`),
			userID, i+1, st.Salt, st.QuestionNonce, st.QuestionCiphertext, st.Proof); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) Rekey(ctx context.Context, userID string, keepSession []byte, e Enrollment) error {
	kdf, err := json.Marshal(e.KDF)
	if err != nil {
		return err
	}
	e, err = s.sealEnrollment(userID, e)
	if err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, s.q(`
			UPDATE {s}.users SET kdf = $2, kdf_salt = $3, auth_hash = $4, wrapped_key = $5, wrapped_nonce = $6,
				failed_unlocks = 0, unlock_lockouts = 0, unlock_locked_until = NULL, updated_at = now()
			WHERE id = $1`),
			userID, kdf, e.KDFSalt, e.AuthKey, e.WrappedKey, e.WrappedNonce); err != nil {
			return err
		}
		if err := s.writeSteps(ctx, tx, userID, e.Steps); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, s.q(`DELETE FROM {s}.sessions WHERE user_id = $1 AND token_hash <> $2`), userID, keepSession)
		return err
	})
}

func (s *Store) scanUser(row pgx.Row) (*User, error) {
	u := &User{}
	var kdf []byte
	err := row.Scan(&u.ID, &u.Username, &kdf, &u.KDFSalt, &u.AuthHash, &u.WrappedKey, &u.WrappedNonce,
		&u.UnlockLockedUntil, &u.UnlockFrozen)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(kdf, &u.KDF); err != nil {
		return nil, err
	}
	return u, s.openUser(u)
}

const userColumns = `id::text, username, kdf, kdf_salt, auth_hash, wrapped_key, wrapped_nonce, unlock_locked_until, unlock_frozen`

func (s *Store) UserByName(ctx context.Context, username string) (*User, error) {
	return s.scanUser(s.pool.QueryRow(ctx, s.q(`SELECT `+userColumns+` FROM {s}.users WHERE username = $1`), username))
}

// UserByNameOrDecoy hands back a fabricated user when the name is unknown, run
// through the same unsealing, so a miss costs what a hit costs.
func (s *Store) UserByNameOrDecoy(ctx context.Context, username string) (*User, bool, error) {
	u, err := s.UserByName(ctx, username)
	if errors.Is(err, ErrNotFound) {
		decoy := *s.decoy
		if err := s.openUser(&decoy); err != nil {
			return nil, false, err
		}
		decoy.KDF = defaultKDF
		return &decoy, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return u, true, nil
}

var defaultKDF = KDF{Algorithm: "argon2id", Iterations: 3, MemoryKiB: 65536, Parallelism: 1}

func (s *Store) UserByID(ctx context.Context, id string) (*User, error) {
	return s.scanUser(s.pool.QueryRow(ctx, s.q(`SELECT `+userColumns+` FROM {s}.users WHERE id = $1`), id))
}

func (s *Store) Step(ctx context.Context, userID string, step int) (*Step, error) {
	st := &Step{}
	err := s.pool.QueryRow(ctx, s.q(`
		SELECT salt, question_nonce, question_ciphertext, proof_hash
		FROM {s}.unlock_steps WHERE user_id = $1 AND step = $2`), userID, step).
		Scan(&st.Salt, &st.QuestionNonce, &st.QuestionCiphertext, &st.Proof)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if st.Salt, err = s.keys.Unseal(stepAAD(userID, step, "salt"), st.Salt); err != nil {
		return nil, err
	}
	if st.QuestionNonce, err = s.keys.Unseal(stepAAD(userID, step, "question_nonce"), st.QuestionNonce); err != nil {
		return nil, err
	}
	st.QuestionCiphertext, err = s.keys.Unseal(stepAAD(userID, step, "question_ciphertext"), st.QuestionCiphertext)
	return st, err
}

type Lockout struct {
	Every       int
	Base        time.Duration
	FreezeAfter int
}

type AnswerResult struct {
	OK           bool
	Frozen       bool
	LockedUntil  *time.Time
	AttemptsLeft int
	FreezesNext  bool
}

func (s *Store) CheckAnswer(ctx context.Context, userID string, step int, proof []byte, l Lockout) (AnswerResult, error) {
	var res AnswerResult
	proofHash := s.keys.Verifier(proof)
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var failures, lockouts int
		var until *time.Time
		var frozen bool
		if err := tx.QueryRow(ctx, s.q(`
			SELECT failed_unlocks, unlock_lockouts, unlock_locked_until, unlock_frozen FROM {s}.users WHERE id = $1 FOR UPDATE`),
			userID).Scan(&failures, &lockouts, &until, &frozen); err != nil {
			return err
		}
		if frozen {
			res.Frozen = true
			return nil
		}
		if until != nil && time.Now().Before(*until) {
			res.LockedUntil = until
			return nil
		}
		var stored []byte
		if err := tx.QueryRow(ctx, s.q(`SELECT proof_hash FROM {s}.unlock_steps WHERE user_id = $1 AND step = $2`), userID, step).
			Scan(&stored); err != nil {
			return err
		}
		if subtle.ConstantTimeCompare(proofHash, stored) == 1 {
			res.OK = true
			return nil
		}
		failures++
		if failures%l.Every == 0 {
			lockouts++
			if lockouts > l.FreezeAfter {
				frozen, until, res.Frozen = true, nil, true
			} else {
				t := time.Now().Add(l.Base << (lockouts - 1))
				until, res.LockedUntil = &t, &t
			}
		}
		res.AttemptsLeft = l.Every - failures%l.Every
		res.FreezesNext = lockouts == l.FreezeAfter
		_, err := tx.Exec(ctx, s.q(`
			UPDATE {s}.users SET failed_unlocks = $2, unlock_lockouts = $3, unlock_locked_until = $4, unlock_frozen = $5
			WHERE id = $1`), userID, failures, lockouts, until, frozen)
		return err
	})
	return res, err
}

// Unfreeze clears a freeze so an operator never has to reach for raw SQL.
func (s *Store) Unfreeze(ctx context.Context, username string) error {
	tag, err := s.pool.Exec(ctx, s.q(`
		UPDATE {s}.users SET failed_unlocks = 0, unlock_lockouts = 0, unlock_locked_until = NULL,
			unlock_frozen = false WHERE username = $1`), username)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ResetUnlockFailures(ctx context.Context, userID string) error {
	_, err := s.pool.Exec(ctx, s.q(`
		UPDATE {s}.users SET failed_unlocks = 0, unlock_lockouts = 0, unlock_locked_until = NULL WHERE id = $1`), userID)
	return err
}

type Session struct {
	UserID      string
	UnlockStage int
	ExpiresAt   time.Time
}

func (s *Store) CreateSession(ctx context.Context, tokenHash []byte, userID string, stage int, ttl time.Duration) error {
	_, err := s.pool.Exec(ctx, s.q(`
		INSERT INTO {s}.sessions (token_hash, user_id, unlock_stage, expires_at)
		VALUES ($1, $2, $3, now() + make_interval(secs => $4))`), tokenHash, userID, stage, ttl.Seconds())
	return err
}

func (s *Store) SessionByToken(ctx context.Context, tokenHash []byte) (*Session, error) {
	sess := &Session{}
	err := s.pool.QueryRow(ctx, s.q(`
		SELECT user_id::text, unlock_stage, expires_at FROM {s}.sessions
		WHERE token_hash = $1 AND expires_at > now()`), tokenHash).Scan(&sess.UserID, &sess.UnlockStage, &sess.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return sess, err
}

func (s *Store) AdvanceUnlock(ctx context.Context, tokenHash []byte, from, to int) (bool, error) {
	tag, err := s.pool.Exec(ctx, s.q(`
		UPDATE {s}.sessions SET unlock_stage = $3 WHERE token_hash = $1 AND unlock_stage = $2`), tokenHash, from, to)
	return tag.RowsAffected() == 1, err
}

func (s *Store) SetUnlockStage(ctx context.Context, tokenHash []byte, stage int) error {
	_, err := s.pool.Exec(ctx, s.q(`UPDATE {s}.sessions SET unlock_stage = $2 WHERE token_hash = $1`), tokenHash, stage)
	return err
}

func (s *Store) FinishUnlock(ctx context.Context, oldHash, newHash []byte) (bool, error) {
	tag, err := s.pool.Exec(ctx, s.q(`
		UPDATE {s}.sessions SET unlock_stage = 3, token_hash = $2 WHERE token_hash = $1 AND unlock_stage = 2`), oldHash, newHash)
	return tag.RowsAffected() == 1, err
}

func (s *Store) RotateSession(ctx context.Context, oldHash, newHash []byte) error {
	tag, err := s.pool.Exec(ctx, s.q(`UPDATE {s}.sessions SET token_hash = $2 WHERE token_hash = $1`), oldHash, newHash)
	if err == nil && tag.RowsAffected() != 1 {
		return ErrNotFound
	}
	return err
}

func (s *Store) DeleteSession(ctx context.Context, tokenHash []byte) error {
	_, err := s.pool.Exec(ctx, s.q(`DELETE FROM {s}.sessions WHERE token_hash = $1`), tokenHash)
	return err
}

func (s *Store) PurgeExpiredSessions(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, s.q(`DELETE FROM {s}.sessions WHERE expires_at <= now()`))
	return err
}

type Item struct {
	ID         string    `json:"id"`
	Kind       string    `json:"kind"`
	Nonce      []byte    `json:"nonce"`
	Ciphertext []byte    `json:"ciphertext"`
	HasFile    bool      `json:"hasFile"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

func (s *Store) ListItems(ctx context.Context, userID string) ([]Item, error) {
	rows, err := s.pool.Query(ctx, s.q(`
		SELECT i.id::text, i.kind, i.nonce, i.ciphertext, f.item_id IS NOT NULL, i.updated_at
		FROM {s}.items i LEFT JOIN {s}.files f ON f.user_id = i.user_id AND f.item_id = i.id
		WHERE i.user_id = $1 ORDER BY i.created_at, i.id`), userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Item, error) {
		var it Item
		err := r.Scan(&it.ID, &it.Kind, &it.Nonce, &it.Ciphertext, &it.HasFile, &it.UpdatedAt)
		return it, err
	})
}

var ErrQuota = errors.New("quota exceeded")

type Quota struct {
	Items int
	Bytes int64
}

func (s *Store) usage(ctx context.Context, tx pgx.Tx, userID, excludeFile string) (items int, bytes int64, err error) {
	if _, err = tx.Exec(ctx, s.q(`SELECT 1 FROM {s}.users WHERE id = $1 FOR UPDATE`), userID); err != nil {
		return
	}
	err = tx.QueryRow(ctx, s.q(`
		SELECT (SELECT count(*) FROM {s}.items WHERE user_id = $1),
			(SELECT coalesce(sum(length(ciphertext)), 0) FROM {s}.items WHERE user_id = $1) +
			(SELECT coalesce(sum(length(blob)), 0) FROM {s}.files WHERE user_id = $1 AND item_id::text <> $2)`),
		userID, excludeFile).Scan(&items, &bytes)
	return
}

var ErrStale = errors.New("stale")

func (s *Store) PutItem(ctx context.Context, userID string, it Item, q Quota, expect []byte) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		items, bytes, err := s.usage(ctx, tx, userID, "")
		if err != nil {
			return err
		}
		var current []byte
		err = tx.QueryRow(ctx, s.q(`SELECT ciphertext FROM {s}.items WHERE user_id = $1 AND id = $2`), userID, it.ID).Scan(&current)
		exists := err == nil
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if expect != nil {
			sum := sha256.Sum256(current)
			if exists == (len(expect) == 0) || (exists && subtle.ConstantTimeCompare(sum[:], expect) != 1) {
				return ErrStale
			}
		}
		previous := int64(len(current))
		if (!exists && items >= q.Items) || bytes-previous+int64(len(it.Ciphertext)) > q.Bytes {
			return ErrQuota
		}
		tag, err := tx.Exec(ctx, s.q(`
			INSERT INTO {s}.items (user_id, id, kind, nonce, ciphertext) VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (user_id, id) DO UPDATE SET nonce = EXCLUDED.nonce, ciphertext = EXCLUDED.ciphertext, updated_at = now()
			WHERE {s}.items.kind = EXCLUDED.kind`), userID, it.ID, it.Kind, it.Nonce, it.Ciphertext)
		if err == nil && tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		return err
	})
}

func (s *Store) DeleteItem(ctx context.Context, userID, id string) error {
	tag, err := s.pool.Exec(ctx, s.q(`DELETE FROM {s}.items WHERE user_id = $1 AND id = $2`), userID, id)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func (s *Store) PutFile(ctx context.Context, userID, itemID string, blob []byte, q Quota) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		_, bytes, err := s.usage(ctx, tx, userID, itemID)
		if err != nil {
			return err
		}
		if bytes+int64(len(blob)) > q.Bytes {
			return ErrQuota
		}
		tag, err := tx.Exec(ctx, s.q(`
			INSERT INTO {s}.files (user_id, item_id, blob)
			SELECT $1, $2, $3 WHERE EXISTS (SELECT 1 FROM {s}.items WHERE user_id = $1 AND id = $2 AND kind = 'file')
			ON CONFLICT (user_id, item_id) DO UPDATE SET blob = EXCLUDED.blob`), userID, itemID, blob)
		if err == nil && tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		return err
	})
}

func (s *Store) File(ctx context.Context, userID, itemID string) ([]byte, error) {
	var blob []byte
	err := s.pool.QueryRow(ctx, s.q(`SELECT blob FROM {s}.files WHERE user_id = $1 AND item_id = $2`), userID, itemID).Scan(&blob)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return blob, err
}

type Passkey struct {
	CredentialID []byte
	UserID       string
	PublicKey    []byte
	AAGUID       []byte
	SignCount    uint32
	BackedUp     bool
	Transports   []string
	Label        string
	CreatedAt    time.Time
	LastUsedAt   *time.Time
}

func (s *Store) AddPasskey(ctx context.Context, p Passkey) error {
	_, err := s.pool.Exec(ctx, s.q(`
		INSERT INTO {s}.passkeys (credential_id, user_id, public_key, aaguid, sign_count, backed_up, transports, label)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`),
		p.CredentialID, p.UserID, p.PublicKey, p.AAGUID, int64(p.SignCount), p.BackedUp, p.Transports, p.Label)
	return err
}

func (s *Store) Passkeys(ctx context.Context, userID string) ([]Passkey, error) {
	rows, err := s.pool.Query(ctx, s.q(`
		SELECT credential_id, user_id::text, public_key, aaguid, sign_count, backed_up, transports, label, created_at, last_used_at
		FROM {s}.passkeys WHERE user_id = $1 ORDER BY created_at`), userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Passkey
	for rows.Next() {
		var p Passkey
		var count int64
		if err := rows.Scan(&p.CredentialID, &p.UserID, &p.PublicKey, &p.AAGUID, &count, &p.BackedUp, &p.Transports, &p.Label, &p.CreatedAt, &p.LastUsedAt); err != nil {
			return nil, err
		}
		p.SignCount = uint32(count)
		out = append(out, p)
	}
	return out, rows.Err()
}

// The door has no account attached yet, so a passkey is looked up by its own id.
func (s *Store) PasskeyByID(ctx context.Context, credentialID []byte) (*Passkey, error) {
	p := &Passkey{}
	var count int64
	err := s.pool.QueryRow(ctx, s.q(`
		SELECT credential_id, user_id::text, public_key, aaguid, sign_count, backed_up, transports, label, created_at, last_used_at
		FROM {s}.passkeys WHERE credential_id = $1`), credentialID).
		Scan(&p.CredentialID, &p.UserID, &p.PublicKey, &p.AAGUID, &count, &p.BackedUp, &p.Transports, &p.Label, &p.CreatedAt, &p.LastUsedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	p.SignCount = uint32(count)
	return p, err
}

func (s *Store) PasskeyUsed(ctx context.Context, credentialID []byte, signCount uint32) error {
	_, err := s.pool.Exec(ctx, s.q(`
		UPDATE {s}.passkeys SET sign_count = $2, last_used_at = now() WHERE credential_id = $1`), credentialID, int64(signCount))
	return err
}

func (s *Store) DeletePasskey(ctx context.Context, userID string, credentialID []byte) error {
	tag, err := s.pool.Exec(ctx, s.q(`DELETE FROM {s}.passkeys WHERE user_id = $1 AND credential_id = $2`), userID, credentialID)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}
