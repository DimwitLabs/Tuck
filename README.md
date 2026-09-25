# Tuck

[![Tuck: all your secrets, tucked in.](landing/og.png)](https://tuck.dimwit.me)

[![CI](https://github.com/DimwitLabs/Tuck/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/DimwitLabs/Tuck/actions/workflows/ci.yml?query=branch%3Amain)
[![Coverage](https://img.shields.io/badge/coverage-85%25-2f567c)](https://github.com/DimwitLabs/Tuck/actions/workflows/ci.yml?query=branch%3Amain)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/DimwitLabs/Tuck/badge)](https://scorecard.dev/viewer/?uri=github.com/DimwitLabs/Tuck)
[![AI-DECLARATION: copilot](https://img.shields.io/badge/䷼%20AI--DECLARATION-copilot-fee2e2?labelColor=fee2e2)](https://ai-declaration.md)
[![Dimwit Pledge](https://dimwit.me/pledge.svg)](https://dimwit.me/pledge)

> [!NOTE]
> This project is backed by the [Dimwit Pledge](https://dimwit.me/pledge).

Tuck is a small, self-hosted vault for SSH keys, hosts and files. One download sets up `~/.ssh` on any machine.

Everything is encrypted in your browser before it's saved, so the server only ever holds ciphertext. You unlock it with a password and then three questions you wrote, and each answer is part of the key.

**[Read the docs →](https://docs.tuck.dimwit.me)**

## Run it

```sh
cp .env.example .env    # set TUCK_SECRET and POSTGRES_PASSWORD
docker compose up -d
```

Or pull the image directly: `docker pull ghcr.io/dimwitlabs/tuck:1`.

Put Tuck behind an HTTPS reverse proxy, set `TUCK_TRUSTED_PROXIES`, then sign up. Each instance has one account, and sign-up closes after the first. The [setup guide](https://docs.tuck.dimwit.me/getting-started/run-it) walks through it, including [nginx](https://docs.tuck.dimwit.me/getting-started/reverse-proxy) and [your own Postgres](https://docs.tuck.dimwit.me/getting-started/your-own-postgres).

## Worth knowing

- **There's no recovery:** Forget the password or an answer and the vault is gone. Keep an export somewhere safe.
- **`TUCK_SECRET` is as precious as the vault:** It seals the salts, questions and wrapped key, so a stolen database is inert without it. Back it up separately from the database, and never in the same dump. Lose it and every vault on the instance is unopenable.
- **Frozen account:** After too many wrong answers, unlocking freezes until you clear it: `docker compose exec tuck tuck unfreeze <username>`.
- **Run it yourself:** Whoever runs the server could change the page and capture what you type. [SECURITY.md](SECURITY.md) and the [threat model](https://docs.tuck.dimwit.me/reference/threat-model) cover the rest.

## Development

```sh
cd web && npm install && npm run build && cd ..
TUCK_SECRET=$(openssl rand -hex 32) TUCK_DATABASE_URL=postgres://tuck:tuck@localhost:5432/tuck TUCK_SECURE_COOKIES=false go run ./cmd/tuck
```

Tests: `npm test` in `web/`, and `go test ./...` with `TUCK_TEST_DATABASE_URL` pointing at a Postgres. Without that variable the Go suite skips almost everything and still prints `ok`, so set it before trusting a green run. The Go suite covers 85% of `internal/`, including a fake authenticator that enrols a passkey and opens the door with it; CI fails if that figure drops. For the number locally:

```sh
go test ./internal/... -coverpkg=./internal/... -coverprofile=cover.out && go tool cover -func=cover.out | tail -1
```

The site lives in `landing/` and `docs/` (`npm start` in `docs/`).

## Licence

MIT
