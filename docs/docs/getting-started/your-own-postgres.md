---
title: your own postgres
description: Point Tuck at Supabase, Neon, RDS or any Postgres over TLS.
---

# your own postgres

already have a postgres? point tuck at it instead of the bundled one.

1. set `TUCK_DATABASE_URL` in `.env`, with `sslmode=require`.
2. start the external-database compose file:

   ```bash
   docker compose -f docker-compose.external-db.yml up -d
   ```

tuck keeps everything in its own `tuck` schema, so it can share a database.

## supabase

use the **session pooler** string from project settings → database:

```ini
TUCK_DATABASE_URL=postgres://postgres.PROJECT_REF:PASSWORD@aws-0-REGION.pooler.supabase.com:5432/postgres?sslmode=require&pool_max_conns=5
```

- not the direct host, which is ipv6 only.
- not the transaction pooler on port 6543, which breaks tuck's queries.
- url-encode symbols in the password: `@` is `%40`, `#` is `%23`.
