---
title: Unlocking
description: The password, the three questions, and what happens after wrong answers.
---

# Unlocking

Every visit is the same: your password, then your three questions, one at a time.

## Wrong answers

Every fifth wrong answer pauses unlocking, twice as long each time:

| Wrong answers | Pause |
|---|---|
| 5 | 15 minutes |
| 10 | 30 minutes |
| 15 | 1 hour |
| 20, 25, 30 | 2, 4, 8 hours |
| 35 | Frozen |

- During a pause, even the right answer is refused.
- A full unlock resets the count.
- `TUCK_FREEZE_AFTER` changes how many pauses come before the freeze.

Nobody can do this to you from outside: answering questions needs a session, so an attacker would already need your password to spend a single wrong answer.

## Frozen?

Whoever runs the instance can lift it:

```bash
docker compose exec tuck /tuck unfreeze <username>
```

Or straight in the database:

```sql
UPDATE tuck.users
SET unlock_frozen = false, failed_unlocks = 0, unlock_lockouts = 0, unlock_locked_until = NULL;
```

## Changing your password or questions

Go to settings. You'll need your current password.
