---
title: Your own Postgres
description: Point Tuck at Supabase, Neon, RDS or any Postgres over TLS.
---

# Your own Postgres

Already have a Postgres? Point Tuck at it instead of the bundled one.

1. Set `TUCK_DATABASE_URL` in `.env`, with `sslmode=require`.
2. Start the external-database compose file:

   ```bash
   docker compose -f docker-compose.external-db.yml up -d
   ```

Tuck keeps everything in its own `tuck` schema, so it can share a database.

## Supabase

Use the **session pooler** string from project settings → database:

```ini
TUCK_DATABASE_URL=postgres://postgres.PROJECT_REF:PASSWORD@aws-0-REGION.pooler.supabase.com:5432/postgres?sslmode=require&pool_max_conns=5
```

- Not the direct host, which is ipv6 only.
- Not the transaction pooler on port 6543, which breaks Tuck's queries.
- Url-encode symbols in the password: `@` is `%40`, `#` is `%23`.
