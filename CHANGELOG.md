# Changelog

All notable changes to Tuck are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.0.0] - 2026-09-26

### Added

- **All Your Secrets, Tucked In:** A vault for SSH keys, hosts and files, encrypted in your browser before any of it is saved. The server only ever holds ciphertext, and never sees your password, your answers, or a single byte of plaintext.
- **Three Questions, One Key:** Log in with a password, then answer three questions you wrote yourself, one at a time and in the order you set. Every answer folds into the key, so a wrong one anywhere opens nothing. There is no reset and no backdoor, which is the point.
- **A Stolen Database Is Inert:** Everything the key derivation needs, the salts, the encrypted questions and the wrapped vault key, is sealed under `TUCK_SECRET`, which lives in your `.env` and not in the database it protects. Without it, a dump carries no way to test a guess against a password or an answer. Tuck refuses to start if the secret is missing or doesn't match the database it is pointed at. Back it up separately and keep it out of your database backups: lose it and every vault on the instance is unopenable.
- **Guessing Gets Expensive:** Wrong answers pause unlocking for longer each time, doubling as they go, and enough of them freeze the account outright. `tuck unfreeze <username>` lifts a freeze when it turns out the culprit was you.
- **Keys, Made Or Brought:** Generate ed25519 or RSA keys in the browser, or paste in the ones you already have.
- **One Line In Your SSH Config:** An install script writes your keys and hosts to `~/.ssh/tuck/` and adds a single `Include` line. Run it again to bring a machine up to date, or `--remove` to undo it. It skips risky ssh options and tells you exactly what it left out.
- **It Notices When Something Is Off:** You get a warning when the server hands back an old copy of an item or file, something you deleted, an older copy of the whole vault, or an empty vault this browser has seen things in. Anything out of date is hidden rather than quietly shown.
- **An Optional Front Door:** With `TUCK_GATE_PASSWORD` set, visitors see one word (`TUCK_GATE_WORD`) and nothing else until they type the password. Set `TUCK_ORIGIN` as well and you can enrol a device that opens the door with its own fingerprint, face or pin. Enrolled devices are named on the door page, so a phone that guesses badly at its own name can be renamed. Nothing ever asks for a passkey on its own: press and hold the door and it asks, so an onlooker never sees a dialog appear out of nowhere.
- **Quality-Of-Life:** The vault locks itself after two quiet minutes, leaving the tab logs you out, reloading tells you why you were logged out, and a copied secret is cleared from the clipboard after 30 seconds. Text, config and image files up to 512 KB open in the tab without downloading, and the decrypted copy lives only as long as the sheet is open.
- **Day And Night:** Light and dark follow your system, with the landing page's sky behind the login steps: sun, clouds and birds by day, stars and a bitten moon by night. A switch on the page and in the vault picks the other one.
- **Yours To Take With You:** A decrypted JSON export, for backups you keep yourself.
- **Run It Where You Like:** A Docker image for amd64 and arm64, with a bundled Postgres that has no network of its own, or point it at your own Postgres over TLS.
- **An Audit Log:** Logins, unlocks, wrong answers, pauses, freezes, gate entries and passkey changes, each with the address it came from.
- **A Landing Page And Documentation:** [tuck.dimwit.me](https://tuck.dimwit.me) and [docs.tuck.dimwit.me](https://docs.tuck.dimwit.me), including a [security page](https://docs.tuck.dimwit.me/reference/security) that spells out what the server can and cannot know, and what a stolen database is actually worth.
