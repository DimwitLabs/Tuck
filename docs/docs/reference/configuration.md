---
title: Configuration
description: Every environment variable Tuck reads, and its default.
---

# Configuration

Everything goes in `.env`, next to the compose file. Every setting has a sensible default.

## The server

| Variable | Default | |
|---|---|---|
| `TUCK_SECRET` | | **Required.** `openssl rand -hex 32`. Seals the salts, questions and wrapped key. Keep it out of your database backups and [back it up separately](../using/backups.md#the-secret) |
| `TUCK_SECRET_FILE` | | Read the secret from a file instead, for Docker secrets |
| `TUCK_FEATURES` | `both` | `ssh`, `files` or `both` |
| `TUCK_TRUSTED_PROXIES` | | Your proxy's address or CIDR range, comma-separated. Required behind a proxy |
| `TUCK_SECURE_COOKIES` | `true` | `false` only for local testing over plain HTTP |
| `TUCK_SESSION_HOURS` | `12` | The longest a login lasts |
| `TUCK_FREEZE_AFTER` | `6` | Timed lockouts before the account freezes |
| `TUCK_MAX_FILE_MB` | `25` | The largest file |
| `TUCK_MAX_ITEMS` | `5000` | Items per account |
| `TUCK_MAX_STORAGE_MB` | `1024` | Storage per account |

## The front door

| Variable | Default | |
|---|---|---|
| `TUCK_GATE_PASSWORD` | | Hides the login page until it's typed. See [the front door](../using/front-door.md) |
| `TUCK_GATE_WORD` | `tuck.` | What the hidden page shows, up to 64 characters |

## The database

| Variable | Default | |
|---|---|---|
| `POSTGRES_PASSWORD` | | The bundled Postgres only. Letters and digits |
| `TUCK_DATABASE_URL` | | Your own Postgres only. See [your own Postgres](../getting-started/your-own-postgres.md) |
| `TUCK_DB_SCHEMA` | `tuck` | The schema Tuck's tables live in |
| `TUCK_DB_ALLOW_PLAINTEXT` | `false` | Allow a remote database without TLS |

## Compose

| Variable | Default | |
|---|---|---|
| `TUCK_VERSION` | `latest` | The image tag to run |
| `TUCK_BIND` | `127.0.0.1` | The address compose publishes Tuck on |
| `TUCK_PORT` | `8080` | The port compose publishes Tuck on |

## Commands

```bash
docker compose exec tuck /tuck assets
```

Prints the SHA-256 of every file Tuck serves, so you can check them against your own build. `/tuck healthcheck` is what Docker's health check runs.

```bash
docker compose exec tuck /tuck unfreeze <username>
```

Lifts a [frozen account](../using/unlocking.md#frozen).
