---
slug: /
title: What Tuck is
sidebar_label: What Tuck is
description: Tuck is a self-hosted vault for SSH keys, hosts and files, encrypted in your browser.
---

# All your secrets, tucked in

Tuck is a small vault for your SSH keys, hosts and files. You run it on your own server, and one download sets up `~/.ssh` on any machine.

Everything is encrypted in your browser. The server only ever sees ciphertext.

## How it works

1. You log in with a password.
2. You answer three questions you wrote, one at a time.
3. Your vault opens. Each answer was part of the key.

Get too many answers wrong and unlocking pauses, then freezes.

## Good to know

- **There's no recovery:** forget an answer and the vault is gone. Keep an export somewhere safe.
- **Run it yourself:** whoever runs the server could tamper with the page it sends you.

## Where to next

- [Run it](./getting-started/run-it.md): up and running in a few minutes.
- [First login](./getting-started/first-login.md): sign up before anyone else can.
