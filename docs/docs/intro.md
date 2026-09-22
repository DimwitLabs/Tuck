---
slug: /
title: what tuck is
sidebar_label: what tuck is
description: Tuck is a self-hosted vault for SSH keys, hosts and files, encrypted in your browser.
---

# all your secrets, tucked in

tuck is a small vault for your ssh keys, hosts and files. you run it on your own server, and one download sets up `~/.ssh` on any machine.

everything is encrypted in your browser. the server only ever sees ciphertext.

## how it works

1. you log in with a password.
2. you answer three questions you wrote, one at a time.
3. your vault opens. each answer was part of the key.

get too many answers wrong and unlocking pauses, then freezes.

## good to know

- **there's no recovery:** forget an answer and the vault is gone. keep an export somewhere safe.
- **run it yourself:** whoever runs the server could tamper with the page it sends you.

## where to next

- [run it](./getting-started/run-it.md): up and running in a few minutes.
- [first login](./getting-started/first-login.md): sign up before anyone else can.
