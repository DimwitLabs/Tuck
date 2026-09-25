---
title: How it's encrypted
description: The key chain from your password and three answers down to each item.
---

# How it's encrypted

Your browser does all the encrypting. The server stores ciphertext and keyed hashes, nothing it could read.

## The chain

Your password and three answers form a chain of keys. Each key comes from the one before, so question 2 stays sealed until answer 1 is right.

| Step | Made from | The server keeps |
|---|---|---|
| Root key | Argon2id of your password, 64 MiB, 3 passes | Nothing |
| Login proof | The root key | An HMAC of it, keyed by `TUCK_SECRET` |
| Each question | Sealed with the key before it: the root key for question 1, then answer 1's key, then answer 2's | The ciphertext, sealed again |
| Each answer's key | Argon2id of the key before it plus your answer | Nothing |
| Each answer's proof | That answer's key | An HMAC of it, keyed by `TUCK_SECRET` |
| Vault key | Random, wrapped by answer 3's key | The wrapped key, sealed again |
| Each item | AES-256-GCM with the vault key | Ciphertext |

The salts are sealed too. So is the one your password is stretched with.

## Sealed in the database

"Sealed" means AES-256-GCM under a key derived from [`TUCK_SECRET`](./configuration.md), which lives in your `.env` rather than in the database. The user id, the step number and the column name go in as additional data, so a row can't be moved between accounts or between questions.

This matters for one attacker in particular: someone holding a copy of the database and nothing else. Without the secret they can't unseal a salt, so they can't even run the key derivation, let alone check whether a guess was right. A stolen dump is inert.

Tuck checks the secret against a sealed value at startup and refuses to run if it doesn't match, rather than failing later in ways nobody can explain.

## Adding up, or multiplying

Worth understanding, because it decides how much your answers are really worth.

**Through the server, the four secrets multiply.** An attacker needs the [front door](../using/front-door.md), then your password for a session, and only then can they try answers, one at a time, in order, against [pauses that double](../using/unlocking.md#wrong-answers). There's no way to test answer two without having answer one.

**With the database and the secret, they add up.** The server has to tell a right answer from a wrong one, or pauses and freezes couldn't exist, and it has to keep question two hidden until answer one lands. Both need something it can check each answer against. Anyone holding both the database and `TUCK_SECRET` can use those to attack the password first, then answer 1, then answer 2, each on its own. That's four modest searches in a row rather than one enormous one, so the strength is roughly your strongest single secret, not all four multiplied.

One guess costs a single Argon2id pass, around 130 ms, and guesses run in parallel across cores. So the sealing is what's carrying the weight here, and answers nobody could look up matter more than clever ones.

## Keeping things current

Every item has a version, listed in an encrypted index. When you unlock, your browser checks both. Old copies, swapped files, deleted things coming back, or a rolled-back vault all get caught and flagged.

## No cheap keys

Your browser refuses key settings weaker than Argon2id with 64 MiB and 3 passes, whatever the server asks for.
