---
title: run it
description: Run Tuck with Docker Compose and the bundled Postgres.
---

# run it

you need docker and a domain with https. that's it.

1. get the files:

   ```bash
   git clone https://github.com/DimwitLabs/Tuck.git && cd Tuck
   cp .env.example .env
   ```

2. set a database password in `.env`:

   ```bash
   openssl rand -hex 24
   ```

3. start it:

   ```bash
   docker compose up -d
   ```

tuck is now on `127.0.0.1:8080`. it won't answer until it's behind https, so set up [the reverse proxy](./reverse-proxy.md) next.

## updating

```bash
docker compose pull && docker compose up -d
```

## picking a version

set `TUCK_VERSION` in `.env`:

- `1` gets fixes, never breaking changes.
- `1.0.0` stays put.

images are for amd64 and arm64, and signed by the workflow that built them:

```bash
gh attestation verify oci://ghcr.io/dimwitlabs/tuck:1.0.0 --owner DimwitLabs
```
