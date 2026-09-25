---
title: what it protects against
description: Who Tuck keeps out, and who it can't.
---

# what it protects against

## covered

- **someone with your password** still needs your three answers, and [wrong answers lock them out](../using/unlocking.md#wrong-answers).
- **someone with the database** has nothing to work with. the salts, questions and wrapped key are [sealed](./security.md#sealing-the-database) under `TUCK_SECRET`, which isn't in the database, so a stolen dump can't even run the key derivation, never mind test a guess.
- **someone who can change the database** can't read or forge anything, and [rollbacks get caught](./how-it-is-encrypted.md#keeping-things-current).
- **someone on the network** sees only tls.
- **someone who knows your username** can't do anything with it. answering questions needs a session, so nobody spends even one wrong answer against your account without your password first. separately, browsers you've logged in from before skip the stranger limit on the login page.
- **someone at your unlocked computer** has two minutes before it locks.
- **someone tampering with your hosts** can't sneak commands onto your machines. the installer [only writes safe options](../using/installing-on-a-machine.md#what-gets-skipped).

## not covered

- **whoever runs the server** could change the page and capture what you type. that's true of every app that encrypts in the browser, which is why tuck is self-hosted.
- **anyone who controls your computer**, through malware or a hostile extension.
- **someone with the database *and* `TUCK_SECRET`.** they can attack your password and each answer separately rather than all at once: [adding up rather than multiplying](./security.md#adding-up-or-multiplying). keep the secret out of your database backups.

## what it assumes about you

you run the host, you keep it patched, and `TUCK_SECRET` lives outside the database and is [backed up separately](../using/backups.md#the-secret). [security](./security.md#what-tuck-assumes-about-you) spells the assumptions out.

## worth knowing

changing your password or questions keeps the same vault key. an old backup plus your old answers still opens it. if both leaked, export and move to a fresh tuck.

## found a hole?

report it privately through [a security advisory](https://github.com/DimwitLabs/Tuck/security/advisories/new) or chief@dimwit.me.
