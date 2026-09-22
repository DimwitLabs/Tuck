---
title: keys and hosts
description: Generate or import SSH keys, and describe the hosts they open.
---

# keys and hosts

## keys

- **generate** ed25519 or rsa keys right in your browser.
- **or paste** a key you already have: openssh, pkcs#1 or pkcs#8.
- give a key a username and hosts using it log in as that user.

## hosts

a host is a line in `ssh_config` with a friendly face: a name, some aliases, the hostname, port, user, key and an optional jump host.

- aliases can't have dots, so nothing can pose as `github.com`.
- search finds names, aliases, hostnames and tags.

## files

anything up to 25 mb, encrypted like everything else. tuck checks each file before it opens it, so a swapped file is refused.
