package store

// Never edit an applied migration; append a new one.
var migrations = []string{
	`
CREATE TABLE {s}.users (
	id                  uuid PRIMARY KEY,
	username            text NOT NULL UNIQUE,
	kdf                 jsonb NOT NULL,
	kdf_salt            bytea NOT NULL,
	auth_hash           bytea NOT NULL,
	wrapped_key         bytea NOT NULL,
	wrapped_nonce       bytea NOT NULL,
	failed_unlocks      integer NOT NULL DEFAULT 0,
	unlock_locked_until timestamptz,
	created_at          timestamptz NOT NULL DEFAULT now(),
	updated_at          timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE {s}.unlock_steps (
	user_id             uuid NOT NULL REFERENCES {s}.users (id) ON DELETE CASCADE,
	step                smallint NOT NULL CHECK (step BETWEEN 1 AND 3),
	salt                bytea NOT NULL,
	question_nonce      bytea NOT NULL,
	question_ciphertext bytea NOT NULL,
	proof_hash          bytea NOT NULL,
	PRIMARY KEY (user_id, step)
);

CREATE TABLE {s}.sessions (
	token_hash   bytea PRIMARY KEY,
	user_id      uuid NOT NULL REFERENCES {s}.users (id) ON DELETE CASCADE,
	unlock_stage smallint NOT NULL DEFAULT 0,
	created_at   timestamptz NOT NULL DEFAULT now(),
	expires_at   timestamptz NOT NULL
);
CREATE INDEX sessions_user_idx ON {s}.sessions (user_id);

CREATE TABLE {s}.items (
	user_id    uuid NOT NULL REFERENCES {s}.users (id) ON DELETE CASCADE,
	id         uuid NOT NULL,
	kind       text NOT NULL CHECK (kind IN ('credential', 'host', 'file')),
	nonce      bytea NOT NULL,
	ciphertext bytea NOT NULL,
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now(),
	PRIMARY KEY (user_id, id)
);

CREATE TABLE {s}.files (
	user_id uuid NOT NULL,
	item_id uuid NOT NULL,
	blob    bytea NOT NULL,
	PRIMARY KEY (user_id, item_id),
	FOREIGN KEY (user_id, item_id) REFERENCES {s}.items (user_id, id) ON DELETE CASCADE
);

CREATE TABLE {s}.settings (
	key   text PRIMARY KEY,
	value bytea NOT NULL
);
`,
	`ALTER TABLE {s}.users ADD COLUMN unlock_lockouts integer NOT NULL DEFAULT 0`,
	`ALTER TABLE {s}.users ADD COLUMN unlock_frozen boolean NOT NULL DEFAULT false`,
	`ALTER TABLE {s}.items DROP CONSTRAINT items_kind_check;
ALTER TABLE {s}.items ADD CONSTRAINT items_kind_check CHECK (kind IN ('credential', 'host', 'file', 'manifest'))`,
	`
CREATE TABLE {s}.passkeys (
	credential_id bytea PRIMARY KEY,
	user_id       uuid NOT NULL REFERENCES {s}.users (id) ON DELETE CASCADE,
	public_key    bytea NOT NULL,
	aaguid        bytea NOT NULL,
	sign_count    bigint NOT NULL DEFAULT 0,
	backed_up     boolean NOT NULL DEFAULT false,
	transports    text[] NOT NULL DEFAULT '{}',
	label         text NOT NULL,
	created_at    timestamptz NOT NULL DEFAULT now(),
	last_used_at  timestamptz
);
CREATE INDEX passkeys_user_idx ON {s}.passkeys (user_id);
`,
}
