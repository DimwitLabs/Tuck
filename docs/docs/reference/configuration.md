---
title: configuration
description: Every environment variable Tuck reads, and its default.
---

# configuration

everything goes in `.env`, next to the compose file. every setting has a sensible default.

## the server

| variable | default | |
|---|---|---|
| `TUCK_FEATURES` | `both` | `ssh`, `files` or `both` |
| `TUCK_TRUSTED_PROXIES` | | your proxy's address or cidr range, comma-separated. required behind a proxy |
| `TUCK_SECURE_COOKIES` | `true` | `false` only for local testing over plain http |
| `TUCK_SESSION_HOURS` | `12` | the longest a login lasts |
| `TUCK_FREEZE_AFTER` | `6` | timed lockouts before the account freezes |
| `TUCK_MAX_FILE_MB` | `25` | the largest file |
| `TUCK_MAX_ITEMS` | `5000` | items per account |
| `TUCK_MAX_STORAGE_MB` | `1024` | storage per account |

## the front door

| variable | default | |
|---|---|---|
| `TUCK_GATE_PASSWORD` | | hides the login page until it's typed. see [the front door](../using/front-door.md) |
| `TUCK_GATE_WORD` | `tuck.` | what the hidden page shows, up to 64 characters |

## the database

| variable | default | |
|---|---|---|
| `POSTGRES_PASSWORD` | | the bundled postgres only. letters and digits |
| `TUCK_DATABASE_URL` | | your own postgres only. see [your own postgres](../getting-started/your-own-postgres.md) |
| `TUCK_DB_SCHEMA` | `tuck` | the schema tuck's tables live in |
| `TUCK_DB_ALLOW_PLAINTEXT` | `false` | allow a remote database without tls |

## compose

| variable | default | |
|---|---|---|
| `TUCK_VERSION` | `latest` | the image tag to run |
| `TUCK_BIND` | `127.0.0.1` | the address compose publishes tuck on |
| `TUCK_PORT` | `8080` | the port compose publishes tuck on |

## commands

```bash
docker compose exec tuck /tuck assets
```

prints the sha-256 of every file tuck serves, so you can check them against your own build. `/tuck healthcheck` is what docker's health check runs.
