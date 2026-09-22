# Changelog

All notable changes to Tuck are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.0.0] - 2026-09-21

### Added

- A vault for SSH keys, hosts and files, encrypted in your browser before it's saved. The server only ever holds ciphertext.
- Login with a password, then three questions you wrote, answered one at a time. Each answer is part of the key.
- Wrong answers pause unlocking for longer each time, and enough of them freeze the account.
- Generate ed25519 or RSA keys in the browser, or paste in ones you already have.
- An install script that puts your keys and hosts in `~/.ssh/tuck/` and adds one `Include` line to your ssh config. Running it again updates the machine, and `--remove` undoes it. It skips risky ssh options and tells you what it left out.
- Warnings when the server hands back an old copy of an item or file, something you deleted, an older copy of the whole vault, or an empty vault this browser has seen things in. Anything out of date is hidden.
- An optional front door: with `TUCK_GATE_PASSWORD` set, visitors see only a word (`TUCK_GATE_WORD`) until they type the password.
- The vault locks after two quiet minutes, leaving the tab logs you out, and copied secrets are cleared from the clipboard after 30 seconds.
- A decrypted JSON export for your own backups.
- A Docker image for amd64 and arm64, with a bundled Postgres that has no network, or your own Postgres over TLS.
- An audit log of logins, unlocks and wrong answers.
- A landing page at [tuck.dimwit.me](https://tuck.dimwit.me) and documentation at [docs.tuck.dimwit.me](https://docs.tuck.dimwit.me).

