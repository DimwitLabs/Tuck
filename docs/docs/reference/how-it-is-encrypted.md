---
title: how it's encrypted
description: The key chain from your password and three answers down to each item.
---

# how it's encrypted

your browser does all the encrypting. the server stores ciphertext and hashes, nothing it could read.

## the chain

your password and three answers form a chain of keys. each key comes from the one before, so question 2 stays sealed until answer 1 is right.

| step | made from | the server keeps |
|---|---|---|
| root key | argon2id of your password, 64 mib, 3 passes | nothing |
| login proof | the root key | a hash |
| each question | sealed with the key before it: the root key for question 1, then answer 1's key, then answer 2's | ciphertext |
| each answer's key | argon2id of the key before it plus your answer | nothing |
| each answer's proof | that answer's key | a hash |
| vault key | random, wrapped by answer 3's key | the wrapped key |
| each item | aes-256-gcm with the vault key | ciphertext |

## keeping things current

every item has a version, listed in an encrypted index. when you unlock, your browser checks both. old copies, swapped files, deleted things coming back, or a rolled-back vault all get caught and flagged.

## no cheap keys

your browser refuses key settings weaker than argon2id with 64 mib and 3 passes, whatever the server asks for.
