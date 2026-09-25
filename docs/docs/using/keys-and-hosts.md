---
title: Keys and hosts
description: Generate or import SSH keys, and describe the hosts they open.
---

# Keys and hosts

## Keys

- **Generate** ed25519 or RSA keys right in your browser.
- **Or paste** a key you already have: openssh, pkcs#1 or pkcs#8.
- Give a key a username and hosts using it log in as that user.

## Hosts

A host is a line in `ssh_config` with a friendly face: a name, some aliases, the hostname, port, user, key and an optional jump host.

- Aliases can't have dots, so nothing can pose as `github.com`.
- Search finds names, aliases, hostnames and tags.

## Files

Anything up to 25 mb, encrypted like everything else. Tuck checks each file before it opens it, so a swapped file is refused.
