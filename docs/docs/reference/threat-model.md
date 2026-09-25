---
title: What it protects against
description: Who Tuck keeps out, and who it can't.
---

# What it protects against

## Covered

- **Someone with your password** still needs your three answers, and [wrong answers lock them out](../using/unlocking.md#wrong-answers).
- **Someone with the database** has nothing to work with. The salts, questions and wrapped key are [sealed](./security.md#sealing-the-database) under `TUCK_SECRET`, which isn't in the database, so a stolen dump can't even run the key derivation, never mind test a guess.
- **Someone who can change the database** can't read or forge anything, and [rollbacks get caught](./how-it-is-encrypted.md#keeping-things-current).
- **Someone on the network** sees only TLS.
- **Someone who knows your username** can't do anything with it. Answering questions needs a session, so nobody spends even one wrong answer against your account without your password first. Separately, browsers you've logged in from before skip the stranger limit on the login page.
- **Someone at your unlocked computer** has two minutes before it locks.
- **Someone tampering with your hosts** can't sneak commands onto your machines. The installer [only writes safe options](../using/installing-on-a-machine.md#what-gets-skipped).

## Not covered

- **Whoever runs the server** could change the page and capture what you type. That's true of every app that encrypts in the browser, which is why Tuck is self-hosted.
- **Anyone who controls your computer**, through malware or a hostile extension.
- **Someone with the database *and* `TUCK_SECRET`.** They can attack your password and each answer separately rather than all at once: [adding up rather than multiplying](./security.md#adding-up-or-multiplying). Keep the secret out of your database backups.

## What it assumes about you

You run the host, you keep it patched, and `TUCK_SECRET` lives outside the database and is [backed up separately](../using/backups.md#the-secret). [security](./security.md#what-tuck-assumes-about-you) spells the assumptions out.

## Worth knowing

Changing your password or questions keeps the same vault key. An old backup plus your old answers still opens it. If both leaked, export and move to a fresh Tuck.

## Found a hole?

Report it privately through [a security advisory](https://github.com/DimwitLabs/Tuck/security/advisories/new) or chief@dimwit.me.
