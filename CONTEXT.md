# Shakedown

Self-hosted recording/streaming app with an admin-facing cloud sync subsystem that backs up recordings to a remote (rclone) target.

## Language

**Failed Sync**:
A `cloud_sync_state` row whose `status = 'error'` — a recording that was claimed and attempted, then gave up after an error, distinct from a recording that has never been claimed (no row) or is still retrying (`status = 'pending'` with a future `next_attempt_at`).
_Avoid_: "sync issue", "broken recording", "ineligible recording"

**Error Class**:
The coarse, closed-vocabulary category of why a Failed Sync happened (`local_missing`, `copy_failed`, `verify_failed`, `lease_expired`), used for grouping and filtering across many failures.
_Avoid_: "error type", "failure reason" (reserve "reason" for the free-text detail)

**Retry Status**:
Whether a Failed Sync is still eligible for automatic retry ("Retrying", `attempts < max_attempts`) or has given up ("Exhausted", `attempts >= max_attempts`) — orthogonal to Error Class, since either outcome can follow from any class.
_Avoid_: "give up", "dead", "stuck"

**Sync Now**:
The existing admin action (`POST /run`) that reconciles _all_ sync candidates in one pass; it only re-claims Failed Syncs still in "Retrying" status — Exhausted ones are silently skipped.
_Avoid_: "reconcile", "run sync" (reserve for internal `Service.Reconcile`)

**Retry**:
A per-recording admin action available on any Failed Sync (regardless of Retry Status) that bypasses the `attempts < max_attempts` claim gate to force one immediate re-attempt; `attempts` still increments normally afterward, so it can exceed `max_attempts` and stays an honest historical count.
_Avoid_: "reset", "resync" (reserve "Sync Now" for the all-candidates action)
