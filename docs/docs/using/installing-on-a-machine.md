---
title: Installing on a machine
description: One script that sets up ~/.SSH from your vault, and how to undo it.
---

# Installing on a machine

1. Open the install tab and download `tuck-install.sh`.
2. Give it a read, then run it:

   ```bash
   sh ~/Downloads/tuck-install.sh
   ```

3. Connect with any alias: `ssh workshop`.

Aliases work everywhere SSH does, so `scp`, `sftp`, `rsync` and `git` pick them up too.

## What it changes

- Keys and hosts go in `~/.ssh/tuck/`.
- One `Include` line goes at the top of `~/.ssh/config`, after a backup.
- The script deletes itself, since it holds your keys.

Run it again any time to update. To undo everything:

```bash
sh ~/Downloads/tuck-install.sh --remove
```

## What gets skipped

Tuck only writes options that tune a connection, like `Port`, `ServerAliveInterval` or `Compression`. A host is skipped if it has:

- An option that runs a command, forwards things or relaxes host-key checks, like `ProxyCommand` or `ForwardAgent`.
- An alias with a dot, or one another host already uses.
- A line break in any field, or a value starting with `-`.

The install tab lists anything it skipped.

<details>
<summary>every option tuck writes</summary>

`AddKeysToAgent` `AddressFamily` `BatchMode` `CheckHostIP` `Ciphers` `Compression` `ConnectionAttempts` `ConnectTimeout` `EscapeChar` `HostKeyAlgorithms` `IdentitiesOnly` `IPQoS` `KbdInteractiveAuthentication` `KexAlgorithms` `LogLevel` `MACs` `NumberOfPasswordPrompts` `PasswordAuthentication` `PreferredAuthentications` `PubkeyAcceptedAlgorithms` `PubkeyAuthentication` `RekeyLimit` `RequestTTY` `ServerAliveCountMax` `ServerAliveInterval` `TCPKeepAlive` `VisualHostKey`

</details>
