# Tuck

[![CI](https://github.com/DimwitLabs/Tuck/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/DimwitLabs/Tuck/actions/workflows/ci.yml?query=branch%3Amain)
[![AI-DECLARATION: copilot](https://img.shields.io/badge/䷼%20AI--DECLARATION-copilot-fee2e2?labelColor=fee2e2)](https://ai-declaration.md)
[![Dimwit Pledge](https://dimwit.me/pledge.svg)](https://dimwit.me/pledge)

> [!NOTE]
> This project is backed by the [Dimwit Pledge](https://dimwit.me/pledge).

A self-hosted vault for SSH keys, hosts and files. One download sets up `~/.ssh` on any machine.

Everything is encrypted in your browser before it's saved. The server only ever holds ciphertext: it never sees your password, your answers or your keys.

## How it works

- You log in with a password, then answer three questions you wrote, one at a time. Each answer is part of the key, and the next question can't even be read until the last answer is right.
- Wrong answers pause unlocking for longer each time: 15 minutes, 30, an hour, and so on. After enough of them the account freezes until whoever runs the database lifts it.
- Leaving or reloading the tab logs you out. Two quiet minutes lock the vault.
- There is no recovery. Forget the password or an answer and the vault is gone. The export tab gives you a decrypted copy for your own backups.

**What it can't protect against:** whoever runs the server could change the page and capture what you type, so run it yourself. And no web app can protect you from a compromised computer. The details are in [SECURITY.md](SECURITY.md).

## Running it

```sh
cp .env.example .env    # set POSTGRES_PASSWORD
docker compose up -d
```

Tuck listens on `127.0.0.1:8080` and expects an HTTPS reverse proxy in front. Put the proxy's address in `TUCK_TRUSTED_PROXIES`, or Tuck refuses every request. For a proxy on the same host, `172.16.0.0/12` usually covers Docker's networks. With nginx:

```nginx
proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
proxy_set_header X-Forwarded-Proto $scheme;
```

Then open it and sign up. **Each instance has one account, and sign-up closes after the first**, so do it before exposing Tuck.

To use your own Postgres (Supabase, Neon, RDS), set `TUCK_DATABASE_URL` with `sslmode=verify-full` or `require`, and use `docker-compose.external-db.yml`.

## Settings

| Variable | Default | |
|---|---|---|
| `TUCK_FEATURES` | `both` | `ssh`, `files` or `both` |
| `TUCK_TRUSTED_PROXIES` | | Your proxy's address or CIDR range |
| `TUCK_GATE_PASSWORD` | | Hides the login page; see below |
| `TUCK_GATE_WORD` | `tuck.` | What the hidden page shows |
| `TUCK_SESSION_HOURS` | `12` | Longest a login lasts |
| `TUCK_FREEZE_AFTER` | `6` | Timed lockouts before the account freezes |
| `TUCK_MAX_FILE_MB` | `25` | Largest file |
| `TUCK_MAX_ITEMS` | `5000` | Items per account |
| `TUCK_MAX_STORAGE_MB` | `1024` | Storage per account |
| `TUCK_SECURE_COOKIES` | `true` | `false` only for local testing over HTTP |
| `TUCK_DATABASE_URL` | | Only with your own Postgres |
| `TUCK_BIND`, `TUCK_PORT` | `127.0.0.1`, `8080` | Where compose publishes Tuck |

## The front door

With `TUCK_GATE_PASSWORD` set, visitors see only the gate word, in the page and in the tab title. Type the password anywhere on the page and the login appears; there's no field and no Enter. Pasting doesn't work. Anyone can guess at it, so make it long.

## Installing on a machine

The install tab builds `tuck-install.sh` from your vault. Download it, read it, then run:

```sh
sh ~/Downloads/tuck-install.sh
```

It puts your keys and hosts in `~/.ssh/tuck/` and adds one `Include` line to the top of `~/.ssh/config`, after backing it up. Running it again brings the machine up to date. It deletes itself afterwards, since it holds your private keys. `--remove` undoes everything.

For safety it skips ssh options that run commands, forward your agent or relax host-key checks, and aliases with dots, which could pose as real hosts like `github.com`. The install tab tells you what it left out.

## If you're frozen out

Whoever runs the database lifts a freeze:

```sql
UPDATE tuck.users
SET unlock_frozen = false, failed_unlocks = 0, unlock_lockouts = 0, unlock_locked_until = NULL;
```

## Backups

The database holds only ciphertext, so a backup is safe to keep anywhere:

```sh
docker compose exec -T db sh -c 'PGPASSWORD=$POSTGRES_PASSWORD pg_dump -U tuck -Fc tuck' > tuck.dump
```

Restore it into a spare instance now and then, and check you can still open it.

`docker compose logs tuck | grep audit` lists logins, unlocks and wrong answers, with the address each came from.

## Development

```sh
cd web && npm install && npm run build && cd ..
TUCK_DATABASE_URL=postgres://tuck:tuck@localhost:5432/tuck TUCK_SECURE_COOKIES=false go run ./cmd/tuck
```

Tests: `npm test` in `web/`, and `go test ./...` with `TUCK_TEST_DATABASE_URL` pointing at a Postgres.

## Licence

MIT
