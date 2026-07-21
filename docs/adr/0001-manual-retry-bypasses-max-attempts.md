# Manual Retry bypasses the max_attempts claim gate

**Status**: accepted

Everywhere else in cloud sync, `attempts < max_attempts` is enforced as an
invariant at the SQL layer (`ClaimNew`'s `WHERE` clause) — once a Failed
Sync's `attempts` reaches `max_attempts` it becomes "Exhausted" and is never
re-claimed automatically. The new per-recording Retry action (added for the
failed-syncs dashboard) deliberately bypasses this gate: it forces one
immediate re-attempt on any Failed Sync regardless of Retry Status, and lets
`attempts` keep incrementing past `max_attempts` afterward rather than
resetting it.

We considered decrementing or resetting `attempts` on Retry instead, so the
`attempts < max_attempts` invariant would stay universally true. We rejected
this because it would make `attempts` a lossy, gate-driven number instead of
an honest historical count of how many times a recording has actually been
attempted — which is more useful for support/debugging ("this recording has
failed 11 times"). The trade-off is that `attempts >= max_attempts` no longer
implies "will never be retried" everywhere in the codebase; it only implies
that *automatic* reclaiming (scheduler, Sync Now) has given up. A future
reader relying on `max_attempts` as a hard ceiling should know this one
explicit, deliberate admin action is the exception.
