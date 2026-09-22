---
title: what it protects against
description: Who Tuck keeps out, and who it can't.
---

# what it protects against

## covered

- **someone with your password** still needs your three answers, and [wrong answers lock them out](../using/unlocking.md#wrong-answers).
- **someone with the database** has to guess offline, and every guess costs a slow argon2id run.
- **someone who can change the database** can't read or forge anything, and [rollbacks get caught](./how-it-is-encrypted.md#keeping-things-current).
- **someone on the network** sees only tls.
- **someone who knows your username** can't lock you out. browsers you've used before skip the stranger limit.
- **someone at your unlocked computer** has two minutes before it locks.
- **someone tampering with your hosts** can't sneak commands onto your machines. the installer [only writes safe options](../using/installing-on-a-machine.md#what-gets-skipped).

## not covered

- **whoever runs the server** could change the page and capture what you type. that's true of every app that encrypts in the browser, which is why tuck is self-hosted.
- **anyone who controls your computer**, through malware or a hostile extension.

## worth knowing

changing your password or questions keeps the same vault key. an old backup plus your old answers still opens it. if both leaked, export and move to a fresh tuck.

## found a hole?

report it privately through [a security advisory](https://github.com/DimwitLabs/Tuck/security/advisories/new) or chief@dimwit.me.
