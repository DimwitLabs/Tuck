---
title: Security
description: What the server is allowed to know, what a stolen database is worth, and what happens to someone guessing.
---

# Security

The short version: everything is encrypted and decrypted in your browser tab. The server holds ciphertext, sealed salts and keyed hashes. It never sees your password, your answers, your keys, or a single byte of plaintext.

This page is the long version, in plain words. [how it's encrypted](./how-it-is-encrypted.md) has the step-by-step chain, and [what it protects against](./threat-model.md) is the short list of who gets in and who doesn't.

## Three doors, in order

Getting to plaintext means passing three separate things. They fail differently.

**The gate:** an optional phrase for the whole instance. The page is deliberately blank: no field, no label, nothing that looks like a login, so a stranger who finds your URL sees nothing worth scraping. It protects the app, not your data: getting through it gets you a login form. See [the front door](../using/front-door.md).

**Login:** username and password. Your browser stretches the password and derives an *auth key*; only that goes to the server. A session at this point can list nothing and read nothing.

**Unlock:** your three questions, one at a time, in a fixed order. Only after the third does your browser hold a key that can decrypt anything.

## The key chain

Your password and three answers are folded into one running value, each fold a full Argon2id pass:

<div className="chain">
  <span>password</span>
  <em>→ argon2id →</em>
  <span>chain</span>
  <em>+ answer 1 → argon2id →</em>
  <span>chain</span>
  <em>+ answer 2 → argon2id →</em>
  <span>chain</span>
  <em>+ answer 3 → argon2id →</em>
  <span className="key">chain</span>
  <em>→ hkdf →</em>
  <span className="key">key that unwraps the vault</span>
</div>

Three things follow:

- **Every input is load-bearing:** there's no partial credit. A wrong input anywhere produces a key that fails the authentication tag, and nothing opens.
- **The order is fixed:** each fold feeds the next, so the answers can't be reordered.
- **Deriving the key isn't free:** four passes over 64 MiB, about half a second on a fast laptop.

Answers are tidied before hashing, trimmed, inner spaces collapsed and lowercased, so capitals and stray spaces are forgiven. Punctuation isn't: `rex.` and `rex` are different answers.

## What the server holds

| Stored | What it is | Any use to a thief with the database? |
|---|---|---|
| The login hash | An HMAC of your auth key, keyed by `TUCK_SECRET` | No, guesses can't be tested without the secret |
| The wrapped vault key | Encrypted under the chain, then sealed | No |
| The salts | Random, one per user and one per question, sealed | No, without them the derivation can't even run |
| Each answer's proof | An HMAC, keyed by `TUCK_SECRET` | No |
| The questions | Encrypted under the chain, then sealed | No, not even the questions can be read |
| Your items | AES-256-GCM under the vault key | No |

Each question is encrypted under the chain as it stands at that point, so the server *cannot* show you question two before answer one is right. That isn't the interface being polite; it's a decryption that can't happen yet.

## How the server checks an answer it can't read

Your browser derives a separate one-way *proof* from the chain at each step and sends that. The server compares it, in constant time, against a keyed hash. The thing on the wire is derived from your answer and useless for working back to it.

That's what makes pauses and freezes possible at all, without the server ever being able to decrypt anything.

## Sealing the database

Everything the derivation needs is sealed with AES-256-GCM under a key derived from [`TUCK_SECRET`](./configuration.md), which lives in your `.env`, not in the database. The account id, step number and column name are bound in, so a row can't be moved between accounts or between questions.

That makes a stolen database inert. No salts to run the derivation with, nothing to check a guess against, no readable questions. Tuck checks the secret against a sealed value at every start and refuses to run if it doesn't match, rather than failing later in ways nobody can explain.

## Adding up, or multiplying

This is the part worth understanding, because it decides what your answers are actually worth.

**Through the server, the four secrets multiply:** an attacker needs the gate, then your password for a session, and only then can they try answers, one at a time, in order, against [pauses that double](../using/unlocking.md#wrong-answers). The wrapped vault key is never sent until all three proofs land. There's no way to test answer two without having answer one.

**With the database *and* the secret, they add up:** the server has to tell a right answer from a wrong one, or the pauses couldn't exist, and it has to keep question two hidden until answer one lands. Both need something it can check each answer against. Anyone holding the database and `TUCK_SECRET` together can use those to attack the password on its own, then answer 1 on its own, and so on: four modest searches in a row rather than one enormous one.

So against that attacker the strength is roughly your strongest single secret, not all four multiplied. A single guess costs one Argon2id pass, around 130 ms, and guesses run in parallel across cores.

There's no clever way around this. Showing one question at a time requires something the server can check each answer against, and anything it can check, a thief holding both halves can check too. The sealing is what keeps the two halves apart, which is why `TUCK_SECRET` doesn't belong in your database backups.

## Blocking guesses

Two separate systems.

**Per-ip limits, held in memory:** broad and cheap, reset when the container restarts. They blunt floods rather than being the real defence.

| Action | Limit | Window |
|---|---|---|
| Login, per IP | 20 | 15 minutes |
| Login, per username | 10 | 15 minutes |
| Answers, per IP | 60 | 15 minutes |
| Gate phrase, per IP | 300 | 15 minutes |
| Sign-up, per IP | 5 | 1 hour |
| Password change, per IP | 5 | 1 hour |

**Per-account pauses, held in the database:** the real one. Wrong *answers* are counted per account and survive restarts. Every fifth one pauses unlocking, each pause twice the last:

| Wrong answers | Paused for | Total wait so far |
|---|---|---|
| 5 | 15 minutes | 15 minutes |
| 10 | 30 minutes | 45 minutes |
| 15 | 1 hour | 1h 45m |
| 20 | 2 hours | 3h 45m |
| 25 | 4 hours | 7h 45m |
| 30 | 8 hours | 15h 45m |
| 35 | Frozen | n/a |

Thirty-five wrong answers freeze the account outright: no timer, no way back without someone [lifting it](../using/unlocking.md#frozen). Deliberate: anyone who has ground through fifteen hours of doubling pauses has earned a human being looking at it. During a pause even the right answer is refused, a full unlock resets the count, and `TUCK_FREEZE_AFTER` moves the threshold.

A few details that come up:

- **Nobody can freeze you out remotely:** answering questions needs a session, so an attacker must already have your password before they can spend a single wrong answer.
- **An unknown username gives nothing away:** it's checked against a decoy that does the same work, so the reply and the time it takes match a wrong password exactly.
- **The gate gives nothing away either:** a wrong phrase gets the same reply as a right one, and only the tail of what you typed is compared, so a stray keystroke first doesn't matter.
- **Everything sensitive is compared in constant time.**
- **Every attempt is logged:** logins, failures, unlocks, wrong answers, pauses, freezes, gate entries and passkey changes, with the client address. See [backups](../using/backups.md#the-audit-log).

## Sessions and locking

A session is a random token; the server keeps only its hash, in an http-only cookie. Sessions carry how far you've unlocked, so a logged-in-but-locked session can't touch vault data, and the token is replaced when you finish unlocking. Sessions expire after 12 hours by default.

The vault key lives in the tab and nowhere else. So locking discards it instantly, and so does reloading, closing the tab or a crash. There's deliberately no "stay unlocked".

## Where passkeys fit

Touch ID and the rest replace the *gate phrase* only. A passkey proves a device has been here before; it can't unlock your vault, because the vault key comes from things that live in your head. Worth saying plainly, since "passkey login" usually means the opposite.

## What Tuck assumes about you

It's self-hosted, so it defends hard against the two things you're actually likely to meet: someone poking at your URL, and a database that gets out: a snapshot, a mounted volume, a dump in the wrong place. It doesn't pretend to defend against your own server being taken over, because then the attacker controls the code your browser runs, and nothing encrypted in a browser survives that.

- **You keep the host patched:** root on the box is the end of it.
- **`TUCK_SECRET` lives outside the database and is backed up separately:** lose it and every vault here is unopenable. Leak it alongside a dump and the sealing is undone. See [backups](../using/backups.md#the-secret).
- **TLS terminates somewhere you trust:** Tuck refuses plain HTTP, so this is your [reverse proxy's](../getting-started/reverse-proxy.md) job.
- **A leaked database backup is bad, but not on its own a break-in:** that's the whole point of sealing.

## What it can't do

- **Whoever runs the server could change the page** and capture what you type. That's true of everything that encrypts in the browser, and it's why Tuck is self-hosted. `tuck assets` prints the hash of every file it serves so you can check them against your own build.
- **Weak answers stay weak:** security questions have little entropy and are often findable. The pauses and the sealing raise the cost a great deal; they can't make a guessable answer unguessable.
- **There's no recovery:** forget the password or an answer and the data is gone. No reset, no backdoor, no escrow. Keep [an export](../using/backups.md#an-export).
- **A computer that's already compromised** sees what you type, and nothing here helps.

## Found a hole?

Report it privately through [a security advisory](https://github.com/DimwitLabs/Tuck/security/advisories/new) or chief@dimwit.me.
