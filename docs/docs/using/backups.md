---
title: backups
description: Back up the database, keep a decrypted export, and read the audit log.
---

# backups

## the database

it's all ciphertext, so the backup is safe to store anywhere:

```bash
docker compose exec -T db sh -c 'PGPASSWORD=$POSTGRES_PASSWORD pg_dump -U tuck -Fc tuck' > tuck.dump
```

restore it somewhere now and then to be sure it works.

## an export

the export tab downloads everything, decrypted, as json. it's your way out if you forget an answer, so keep it as safe as the keys themselves.

## the audit log

logins, unlocks and wrong answers, with where they came from:

```bash
docker compose logs tuck | grep audit
```
