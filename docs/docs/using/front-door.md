---
title: the front door
description: Hide the login page behind a single word until someone types the password.
---

# the front door

want tuck to look like nothing's there? set a gate password:

```ini
TUCK_GATE_PASSWORD=four random words you can type
TUCK_GATE_WORD=hello.
```

visitors now see one word. type the password anywhere on the page, no field and no enter, and the login appears.

- pasting doesn't work, so pick something you can type.
- guesses are rate-limited, but make it long anyway.
- once you're in, the browser is let through for 15 minutes.

it hides tuck from passers-by. your password and answers are what keep the vault safe.
