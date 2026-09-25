---
title: security
description: What the server is allowed to know, what a stolen database is worth, and what happens to someone guessing.
---

# security

the short version: everything is encrypted and decrypted in your browser tab. the server holds ciphertext, sealed salts and keyed hashes. it never sees your password, your answers, your keys, or a single byte of plaintext.

this page is the long version, in plain words. [how it's encrypted](./how-it-is-encrypted.md) has the step-by-step chain, and [what it protects against](./threat-model.md) is the short list of who gets in and who doesn't.

## three doors, in order

getting to plaintext means passing three separate things. they fail differently.

**the gate:** an optional phrase for the whole instance. the page is deliberately blank — no field, no label, nothing that looks like a login — so a stranger who finds your url sees nothing worth scraping. it protects the app, not your data: getting through it gets you a login form. see [the front door](../using/front-door.md).

**login:** username and password. your browser stretches the password and derives an *auth key*; only that goes to the server. a session at this point can list nothing and read nothing.

**unlock:** your three questions, one at a time, in a fixed order. only after the third does your browser hold a key that can decrypt anything.

## the key chain

your password and three answers are folded into one running value, each fold a full argon2id pass:

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

three things follow:

- **every input is load-bearing:** there's no partial credit. a wrong input anywhere produces a key that fails the authentication tag, and nothing opens.
- **the order is fixed:** each fold feeds the next, so the answers can't be reordered.
- **deriving the key isn't free:** four passes over 64 mib, about half a second on a fast laptop.

answers are tidied before hashing — trimmed, inner spaces collapsed, lowercased — so capitals and stray spaces are forgiven. punctuation isn't: `rex.` and `rex` are different answers.

## what the server holds

| stored | what it is | any use to a thief with the database? |
|---|---|---|
| the login hash | an hmac of your auth key, keyed by `TUCK_SECRET` | no — guesses can't be tested without the secret |
| the wrapped vault key | encrypted under the chain, then sealed | no |
| the salts | random, one per user and one per question, sealed | no — without them the derivation can't even run |
| each answer's proof | an hmac, keyed by `TUCK_SECRET` | no |
| the questions | encrypted under the chain, then sealed | no — not even the questions can be read |
| your items | aes-256-gcm under the vault key | no |

each question is encrypted under the chain as it stands at that point, so the server *cannot* show you question two before answer one is right. that isn't the interface being polite; it's a decryption that can't happen yet.

## how the server checks an answer it can't read

your browser derives a separate one-way *proof* from the chain at each step and sends that. the server compares it, in constant time, against a keyed hash. the thing on the wire is derived from your answer and useless for working back to it.

that's what makes pauses and freezes possible at all, without the server ever being able to decrypt anything.

## sealing the database

everything the derivation needs is sealed with aes-256-gcm under a key derived from [`TUCK_SECRET`](./configuration.md), which lives in your `.env`, not in the database. the account id, step number and column name are bound in, so a row can't be moved between accounts or between questions.

that makes a stolen database inert. no salts to run the derivation with, nothing to check a guess against, no readable questions. tuck checks the secret against a sealed value at every start and refuses to run if it doesn't match, rather than failing later in ways nobody can explain.

## adding up, or multiplying

this is the part worth understanding, because it decides what your answers are actually worth.

**through the server, the four secrets multiply:** an attacker needs the gate, then your password for a session, and only then can they try answers — one at a time, in order, against [pauses that double](../using/unlocking.md#wrong-answers). the wrapped vault key is never sent until all three proofs land. there's no way to test answer two without having answer one.

**with the database *and* the secret, they add up:** the server has to tell a right answer from a wrong one, or the pauses couldn't exist, and it has to keep question two hidden until answer one lands. both need something it can check each answer against. anyone holding the database and `TUCK_SECRET` together can use those to attack the password on its own, then answer 1 on its own, and so on — four modest searches in a row rather than one enormous one.

so against that attacker the strength is roughly your strongest single secret, not all four multiplied. a single guess costs one argon2id pass, around 130 ms, and guesses run in parallel across cores.

there's no clever way around this. showing one question at a time requires something the server can check each answer against, and anything it can check, a thief holding both halves can check too. the sealing is what keeps the two halves apart, which is why `TUCK_SECRET` doesn't belong in your database backups.

## blocking guesses

two separate systems.

**per-ip limits, held in memory:** broad and cheap, reset when the container restarts. they blunt floods rather than being the real defence.

| action | limit | window |
|---|---|---|
| login, per ip | 20 | 15 minutes |
| login, per username | 10 | 15 minutes |
| answers, per ip | 60 | 15 minutes |
| gate phrase, per ip | 300 | 15 minutes |
| sign-up, per ip | 5 | 1 hour |
| password change, per ip | 5 | 1 hour |

**per-account pauses, held in the database:** the real one. wrong *answers* are counted per account and survive restarts. every fifth one pauses unlocking, each pause twice the last:

| wrong answers | paused for | total wait so far |
|---|---|---|
| 5 | 15 minutes | 15 minutes |
| 10 | 30 minutes | 45 minutes |
| 15 | 1 hour | 1h 45m |
| 20 | 2 hours | 3h 45m |
| 25 | 4 hours | 7h 45m |
| 30 | 8 hours | 15h 45m |
| 35 | frozen | — |

thirty-five wrong answers freeze the account outright: no timer, no way back without someone [lifting it](../using/unlocking.md#frozen). deliberate — anyone who has ground through fifteen hours of doubling pauses has earned a human being looking at it. during a pause even the right answer is refused, a full unlock resets the count, and `TUCK_FREEZE_AFTER` moves the threshold.

a few details that come up:

- **nobody can freeze you out remotely:** answering questions needs a session, so an attacker must already have your password before they can spend a single wrong answer.
- **an unknown username gives nothing away:** it's checked against a decoy that does the same work, so the reply and the time it takes match a wrong password exactly.
- **the gate gives nothing away either:** a wrong phrase gets the same reply as a right one, and only the tail of what you typed is compared, so a stray keystroke first doesn't matter.
- **everything sensitive is compared in constant time.**
- **every attempt is logged:** logins, failures, unlocks, wrong answers, pauses, freezes, gate entries and passkey changes, with the client address. see [backups](../using/backups.md#the-audit-log).

## sessions and locking

a session is a random token; the server keeps only its hash, in an http-only cookie. sessions carry how far you've unlocked, so a logged-in-but-locked session can't touch vault data, and the token is replaced when you finish unlocking. sessions expire after 12 hours by default.

the vault key lives in the tab and nowhere else. so locking discards it instantly, and so does reloading, closing the tab or a crash. there's deliberately no "stay unlocked".

## where passkeys fit

touch id and the rest replace the *gate phrase* only. a passkey proves a device has been here before; it can't unlock your vault, because the vault key comes from things that live in your head. worth saying plainly, since "passkey login" usually means the opposite.

## what tuck assumes about you

it's self-hosted, so it defends hard against the two things you're actually likely to meet: someone poking at your url, and a database that gets out — a snapshot, a mounted volume, a dump in the wrong place. it doesn't pretend to defend against your own server being taken over, because then the attacker controls the code your browser runs, and nothing encrypted in a browser survives that.

- **you keep the host patched:** root on the box is the end of it.
- **`TUCK_SECRET` lives outside the database and is backed up separately:** lose it and every vault here is unopenable. leak it alongside a dump and the sealing is undone. see [backups](../using/backups.md#the-secret).
- **tls terminates somewhere you trust:** tuck refuses plain http, so this is your [reverse proxy's](../getting-started/reverse-proxy.md) job.
- **a leaked database backup is bad, but not on its own a break-in:** that's the whole point of sealing.

## what it can't do

- **whoever runs the server could change the page** and capture what you type. that's true of everything that encrypts in the browser, and it's why tuck is self-hosted. `tuck assets` prints the hash of every file it serves so you can check them against your own build.
- **weak answers stay weak:** security questions have little entropy and are often findable. the pauses and the sealing raise the cost a great deal; they can't make a guessable answer unguessable.
- **there's no recovery:** forget the password or an answer and the data is gone. no reset, no backdoor, no escrow. keep [an export](../using/backups.md#an-export).
- **a computer that's already compromised** sees what you type, and nothing here helps.

## found a hole?

report it privately through [a security advisory](https://github.com/DimwitLabs/Tuck/security/advisories/new) or chief@dimwit.me.
