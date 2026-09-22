---
title: unlocking
description: The password, the three questions, and what happens after wrong answers.
---

# unlocking

every visit is the same: your password, then your three questions, one at a time.

## wrong answers

every fifth wrong answer pauses unlocking, twice as long each time:

| wrong answers | pause |
|---|---|
| 5 | 15 minutes |
| 10 | 30 minutes |
| 15 | 1 hour |
| 20, 25, 30 | 2, 4, 8 hours |
| 35 | frozen |

- during a pause, even the right answer is refused.
- a full unlock resets the count.
- `TUCK_FREEZE_AFTER` changes how many pauses come before the freeze.

## frozen?

whoever runs the database can lift it:

```sql
UPDATE tuck.users
SET unlock_frozen = false, failed_unlocks = 0, unlock_lockouts = 0, unlock_locked_until = NULL;
```

## changing your password or questions

go to settings. you'll need your current password.
