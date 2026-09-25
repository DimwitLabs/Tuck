---
title: how it's encrypted
description: The key chain from your password and three answers down to each item.
---

# how it's encrypted

your browser does all the encrypting. the server stores ciphertext and keyed hashes, nothing it could read.

## the chain

your password and three answers form a chain of keys. each key comes from the one before, so question 2 stays sealed until answer 1 is right.

| step | made from | the server keeps |
|---|---|---|
| root key | argon2id of your password, 64 mib, 3 passes | nothing |
| login proof | the root key | an hmac of it, keyed by `TUCK_SECRET` |
| each question | sealed with the key before it: the root key for question 1, then answer 1's key, then answer 2's | the ciphertext, sealed again |
| each answer's key | argon2id of the key before it plus your answer | nothing |
| each answer's proof | that answer's key | an hmac of it, keyed by `TUCK_SECRET` |
| vault key | random, wrapped by answer 3's key | the wrapped key, sealed again |
| each item | aes-256-gcm with the vault key | ciphertext |

the salts are sealed too. so is the one your password is stretched with.

## sealed in the database

"sealed" means aes-256-gcm under a key derived from [`TUCK_SECRET`](./configuration.md), which lives in your `.env` rather than in the database. the user id, the step number and the column name go in as additional data, so a row can't be moved between accounts or between questions.

this matters for one attacker in particular: someone holding a copy of the database and nothing else. without the secret they can't unseal a salt, so they can't even run the key derivation, let alone check whether a guess was right. a stolen dump is inert.

tuck checks the secret against a sealed value at startup and refuses to run if it doesn't match, rather than failing later in ways nobody can explain.

## adding up, or multiplying

worth understanding, because it decides how much your answers are really worth.

**through the server, the four secrets multiply.** an attacker needs the [front door](../using/front-door.md), then your password for a session, and only then can they try answers — one at a time, in order, against [pauses that double](../using/unlocking.md#wrong-answers). there's no way to test answer two without having answer one.

**with the database and the secret, they add up.** the server has to tell a right answer from a wrong one, or pauses and freezes couldn't exist, and it has to keep question two hidden until answer one lands. both need something it can check each answer against. anyone holding both the database and `TUCK_SECRET` can use those to attack the password first, then answer 1, then answer 2, each on its own. that's four modest searches in a row rather than one enormous one, so the strength is roughly your strongest single secret, not all four multiplied.

one guess costs a single argon2id pass, around 130 ms, and guesses run in parallel across cores. so the sealing is what's carrying the weight here, and answers nobody could look up matter more than clever ones.

## keeping things current

every item has a version, listed in an encrypted index. when you unlock, your browser checks both. old copies, swapped files, deleted things coming back, or a rolled-back vault all get caught and flagged.

## no cheap keys

your browser refuses key settings weaker than argon2id with 64 mib and 3 passes, whatever the server asks for.
