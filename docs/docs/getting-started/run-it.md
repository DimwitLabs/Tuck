---
title: Run it
description: Run Tuck with Docker Compose and the bundled Postgres.
---

# Run it

You need Docker and a domain with HTTPS. That's it.

1. Get the files:

   ```bash
   git clone https://github.com/DimwitLabs/Tuck.git && cd Tuck
   cp .env.example .env
   ```

2. Set `TUCK_SECRET` and a database password in `.env`:

   ```bash
   openssl rand -hex 32    # TUCK_SECRET
   openssl rand -hex 24    # POSTGRES_PASSWORD
   ```

   `TUCK_SECRET` seals what's in the database. [back it up separately](../using/backups.md#the-secret) — lose it and every vault here is unopenable.

3. Start it:

   ```bash
   docker compose up -d
   ```

Tuck is now on `127.0.0.1:8080`. It won't answer until it's behind HTTPS, so set up [the reverse proxy](./reverse-proxy.md) next.

## Updating

```bash
docker compose pull && docker compose up -d
```

## Picking a version

Set `TUCK_VERSION` in `.env`:

- `1` gets fixes, never breaking changes.
- `1.0.0` stays put.

Images are for amd64 and arm64, and signed by the workflow that built them:

```bash
gh attestation verify oci://ghcr.io/dimwitlabs/tuck:1.0.0 --owner DimwitLabs
```
