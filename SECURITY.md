# Security

## Found something?

Please tell us privately, either through **Security → Report a vulnerability** on this repository or by email to chief@dimwit.me. Don't open a public issue.

## How your data is locked

Everything is encrypted in your browser. The server only ever stores ciphertext and hashes, so a copy of the database on its own gives nothing away.

Your password and your three answers form a chain. Each link is derived from the one before it, and each question is encrypted with the key from the answer before it, so nobody can even read question two until answer one is right.

| Step | Made from | The server keeps |
|---|---|---|
| Root key | Argon2id of your password (64 MiB, 3 passes) | Nothing |
| Login proof | Derived from the root key | Its SHA-256 |
| Question *i* | Encrypted with a key from the previous link | The ciphertext |
| Link *i* | Argon2id of the previous link plus answer *i* | Nothing |
| Answer proof *i* | Derived from link *i* | Its SHA-256 |
| Vault key | Random, wrapped with a key from the last link | The wrapped key |
| Each item | AES-256-GCM with the vault key, bound to its id, kind and version | The ciphertext |

The server hands out the next question only after checking the previous answer's proof, and hands over the wrapped vault key only after the third.

## Who it protects you from

**Someone who knows your password** still has to guess your answers through the server, one at a time. Every few wrong guesses pause unlocking for longer, and eventually the account freezes until whoever runs the database lifts it.

**Someone who steals the database** can guess offline, but every guess costs a slow, memory-hungry Argon2id run. Because answers are checked one by one, the cost adds up rather than multiplying, so answers nobody could look up matter more than clever ones.

**Someone who can change the database** can't read or forge anything. If they put back an old copy of an item, an old version of a file, something you deleted, or roll back the whole vault, your browser notices and tells you.

**Someone on the network** sees only TLS, since Tuck refuses plain HTTP. The bundled Postgres has no network at all; Tuck reaches it through a password-protected socket.

**Someone who knows your username** can't lock you out. Browsers you've logged in from before skip the limit that strangers run into.

**Someone at your unlocked computer** has two quiet minutes before it locks, and leaving the tab logs you out. Changing your password needs the current one, and a few wrong tries end the session.

**Someone who tampers with your vault** to get at the machines you install it on is stopped by the installer, which only writes harmless ssh options and refuses aliases that could pose as real hosts like `github.com`.

## Who it can't protect you from

**Whoever runs the server:** They could change the page it sends you and capture what you type. That's true of every web app that encrypts in the browser, which is why Tuck is meant to be self-hosted. `tuck assets` prints the hash of every file it serves, so you can compare them with your own build.

**Anyone who controls your computer:** Through malware, a hostile browser extension, or someone watching over your shoulder.

## Worth knowing

Changing your password or questions re-wraps the same vault key, so an old backup together with your old password and answers still opens things saved later. If you think both have leaked, export everything and move to a fresh instance.

There's intentionally no recovery.
