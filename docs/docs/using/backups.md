---
title: Backups
description: Back up the secret and the database, keep a decrypted export, and read the audit log.
---

# Backups

## The secret

`TUCK_SECRET`, from your `.env`, is what seals the salts, questions and wrapped key in the database. Back it up **somewhere other than your database backups**, and keep it there:

- Without it, a database backup is unopenable. Restoring one onto an instance with a different secret won't work, by design.
- With it, someone holding a database backup is back to guessing your password and answers [one at a time](../reference/security.md#adding-up-or-multiplying).

A password manager or an envelope in a drawer is fine. The same place as `tuck.dump` is not.

## The database

It's all ciphertext and sealed values, so a backup on its own gives nothing away:

```bash
docker compose exec -T db sh -c 'PGPASSWORD=$POSTGRES_PASSWORD pg_dump -U tuck -Fc tuck' > tuck.dump
```

Restore it somewhere now and then to be sure it works — with the same `TUCK_SECRET`, or it won't open.

## An export

The export tab downloads everything, decrypted, as JSON. It's your way out if you forget an answer, so keep it as safe as the keys themselves.

## The audit log

Logins, unlocks and wrong answers, with where they came from:

```bash
docker compose logs tuck | grep audit
```
