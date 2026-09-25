---
title: backups
description: Back up the secret and the database, keep a decrypted export, and read the audit log.
---

# backups

## the secret

`TUCK_SECRET`, from your `.env`, is what seals the salts, questions and wrapped key in the database. back it up **somewhere other than your database backups**, and keep it there:

- without it, a database backup is unopenable. restoring one onto an instance with a different secret won't work, by design.
- with it, someone holding a database backup is back to guessing your password and answers [one at a time](../reference/security.md#adding-up-or-multiplying).

a password manager or an envelope in a drawer is fine. the same place as `tuck.dump` is not.

## the database

it's all ciphertext and sealed values, so a backup on its own gives nothing away:

```bash
docker compose exec -T db sh -c 'PGPASSWORD=$POSTGRES_PASSWORD pg_dump -U tuck -Fc tuck' > tuck.dump
```

restore it somewhere now and then to be sure it works — with the same `TUCK_SECRET`, or it won't open.

## an export

the export tab downloads everything, decrypted, as json. it's your way out if you forget an answer, so keep it as safe as the keys themselves.

## the audit log

logins, unlocks and wrong answers, with where they came from:

```bash
docker compose logs tuck | grep audit
```
