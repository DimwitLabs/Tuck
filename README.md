# Tuck

[![Tuck: all your secrets, tucked in.](landing/og.png)](https://tuck.dimwit.me)

[![CI](https://github.com/DimwitLabs/Tuck/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/DimwitLabs/Tuck/actions/workflows/ci.yml?query=branch%3Amain)
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
cp .env.example .env    # set POSTGRES_PASSWORD
docker compose up -d
```

Or pull the image directly: `docker pull ghcr.io/dimwitlabs/tuck:1`.

Put Tuck behind an HTTPS reverse proxy, set `TUCK_TRUSTED_PROXIES`, then sign up. Each instance has one account, and sign-up closes after the first. The [setup guide](https://docs.tuck.dimwit.me/getting-started/run-it) walks through it, including [nginx](https://docs.tuck.dimwit.me/getting-started/reverse-proxy) and [your own Postgres](https://docs.tuck.dimwit.me/getting-started/your-own-postgres).

## Worth knowing

- **There's no recovery:** Forget the password or an answer and the vault is gone. Keep an export somewhere safe.
- **Run it yourself:** Whoever runs the server could change the page and capture what you type. [SECURITY.md](SECURITY.md) and the [threat model](https://docs.tuck.dimwit.me/reference/threat-model) cover the rest.

## Development

```sh
cd web && npm install && npm run build && cd ..
TUCK_DATABASE_URL=postgres://tuck:tuck@localhost:5432/tuck TUCK_SECURE_COOKIES=false go run ./cmd/tuck
```

Tests: `npm test` in `web/`, and `go test ./...` with `TUCK_TEST_DATABASE_URL` pointing at a Postgres. The site lives in `landing/` and `docs/` (`npm start` in `docs/`).

## Licence

MIT
