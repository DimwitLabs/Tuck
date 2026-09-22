---
title: installing on a machine
description: One script that sets up ~/.ssh from your vault, and how to undo it.
---

# installing on a machine

1. open the install tab and download `tuck-install.sh`.
2. give it a read, then run it:

   ```bash
   sh ~/Downloads/tuck-install.sh
   ```

3. connect with any alias: `ssh workshop`.

aliases work everywhere ssh does, so `scp`, `sftp`, `rsync` and `git` pick them up too.

## what it changes

- keys and hosts go in `~/.ssh/tuck/`.
- one `Include` line goes at the top of `~/.ssh/config`, after a backup.
- the script deletes itself, since it holds your keys.

run it again any time to update. to undo everything:

```bash
sh ~/Downloads/tuck-install.sh --remove
```

## what gets skipped

tuck only writes options that tune a connection, like `Port`, `ServerAliveInterval` or `Compression`. a host is skipped if it has:

- an option that runs a command, forwards things or relaxes host-key checks, like `ProxyCommand` or `ForwardAgent`.
- an alias with a dot, or one another host already uses.
- a line break in any field, or a value starting with `-`.

the install tab lists anything it skipped.

<details>
<summary>every option tuck writes</summary>

`AddKeysToAgent` `AddressFamily` `BatchMode` `CheckHostIP` `Ciphers` `Compression` `ConnectionAttempts` `ConnectTimeout` `EscapeChar` `HostKeyAlgorithms` `IdentitiesOnly` `IPQoS` `KbdInteractiveAuthentication` `KexAlgorithms` `LogLevel` `MACs` `NumberOfPasswordPrompts` `PasswordAuthentication` `PreferredAuthentications` `PubkeyAcceptedAlgorithms` `PubkeyAuthentication` `RekeyLimit` `RequestTTY` `ServerAliveCountMax` `ServerAliveInterval` `TCPKeepAlive` `VisualHostKey`

</details>
