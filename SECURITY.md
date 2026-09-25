# Security

## Found something?

Please tell us privately, either through **Security → Report a vulnerability** on this repository or by email to chief@dimwit.me. Don't open a public issue.

## How your data is locked

Everything is encrypted in your browser. The server only ever stores ciphertext, sealed values and keyed hashes, so a copy of the database on its own gives nothing away.

Your password and your three answers form a chain. Each link is derived from the one before it, and each question is encrypted with the key from the answer before it, so nobody can even read question two until answer one is right.

| Step | Made from | The server keeps |
|---|---|---|
| Root key | Argon2id of your password (64 MiB, 3 passes) | Nothing |
| Login proof | Derived from the root key | An HMAC of it, keyed by `TUCK_SECRET` |
| Each question | Encrypted with the key before it: the root key for question 1, then answer 1's key, then answer 2's | The ciphertext, sealed again |
| Each answer's key | Argon2id of the key before it plus your answer | Nothing |
| Each answer's proof | Derived from that answer's key | An HMAC of it, keyed by `TUCK_SECRET` |
| Vault key | Random, wrapped with answer 3's key | The wrapped key, sealed again |
| Each item | AES-256-GCM with the vault key, bound to its id, kind and version | The ciphertext |

The salts are sealed too. "Sealed" means AES-256-GCM under a key derived from `TUCK_SECRET`, which lives in your `.env` rather than in the database, with the account id, step number and column name bound in so a row can't be moved between accounts or questions.

The server hands out the next question only after checking the previous answer's proof, and hands over the wrapped vault key only after the third.

## Who it protects you from

**Someone who knows your password** still has to guess your answers through the server, one at a time. Every few wrong guesses pause unlocking for longer, and eventually the account freezes until whoever runs the database lifts it.

**Someone who steals the database** has nothing to work with. Without `TUCK_SECRET` they can't unseal a salt, so they can't run the key derivation at all, let alone check whether a guess was right. Tuck verifies the secret against a sealed value at startup and refuses to run if it doesn't match.

**Someone who steals the database *and* the secret** is back to guessing offline, and there the answers add up rather than multiplying. The server must be able to tell a right answer from a wrong one, otherwise the pauses and freezes couldn't exist, and it must keep question two hidden until answer one lands. So it stores something it can check each answer against. Anyone with both halves can use those to attack the password on its own, then each answer on its own: four modest searches in sequence rather than one enormous one. Effective strength is roughly your strongest single secret, so answers nobody could look up matter more than clever ones, and the secret does not belong in your database backups.

**Someone who can change the database** can't read or forge anything. If they put back an old copy of an item, an old version of a file, something you deleted, or roll back the whole vault, your browser notices and tells you.

**Someone on the network** sees only TLS, since Tuck refuses plain HTTP. The bundled Postgres has no network at all; Tuck reaches it through a password-protected socket.

**Someone who knows your username** can't do anything with it. Answering questions requires a session, so nobody can spend even one wrong answer against your account without your password first. There is no way to freeze someone out remotely. Separately, browsers you've logged in from before skip the login limit that strangers run into.

**Someone at your unlocked computer** has two quiet minutes before it locks, and leaving the tab logs you out. Changing your password needs the current one, and a few wrong tries end the session.

**Someone who tampers with your vault** to get at the machines you install it on is stopped by the installer, which only writes harmless ssh options and refuses aliases that could pose as real hosts like `github.com`.

## Who it can't protect you from

**Whoever runs the server:** They could change the page it sends you and capture what you type. That's true of every web app that encrypts in the browser, which is why Tuck is meant to be self-hosted. `tuck assets` prints the hash of every file it serves, so you can compare them with your own build.

**Anyone who controls your computer:** Through malware, a hostile browser extension, or someone watching over your shoulder.

## Worth knowing

Changing your password or questions re-wraps the same vault key, so an old backup together with your old password and answers still opens things saved later. If you think both have leaked, export everything and move to a fresh instance.

**`TUCK_SECRET` is as precious as the vault.** Lose it and every account on the instance is unopenable, with no way back. Back it up somewhere other than your database backups. There is no rotation yet: changing it means everyone enrols again.

There's intentionally no recovery.

The full design, in plain words, is on [the security page](https://docs.tuck.dimwit.me/reference/security).
