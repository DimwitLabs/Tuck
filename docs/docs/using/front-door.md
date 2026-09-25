---
title: The front door
description: Hide the login page behind a single word until someone types the password.
---

# The front door

Want Tuck to look like nothing's there? Set a gate password:

```ini
TUCK_GATE_PASSWORD=four random words you can type
TUCK_GATE_WORD=hello.
```

Visitors now see one word. Type the password anywhere on the page, no field and no enter, and the login appears.

- Pasting doesn't work, so pick something you can type.
- Guesses are rate-limited, but make it long anyway.
- Once you're in, the browser is let through for 15 minutes.

It hides Tuck from passers-by. Your password and answers are what keep the vault safe.
