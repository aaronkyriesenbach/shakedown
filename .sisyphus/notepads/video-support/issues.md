# Issues & Gotchas — video-support

## 2026-05-10 — Atlas Setup

### Known Gotchas

- **ftyp brand parsing**: Current validation uses hardcoded box sizes (0x20, 0x1C). The real fix parses bytes 8-11 dynamically (after `size(4) + "ftyp"(4)`). The size field (bytes 0-3) is variable — do NOT match it.
- **No-audio video**: Screen recordings may lack audio track. Must handle gracefully in ffprobe parsing (set sampleRate=0, channels=0).
- **Short video**: 10% of <1s rounds to 0s for thumbnail. Clamp to minimum 0.1s.
- **processing_step CHECK constraint**: Must drop old constraint from migration 002 and re-add with new values. Migration 002 used `ALTER TABLE ... ADD COLUMN ... CHECK(...)`. Migration 004 drops the old check and adds a new one via `ALTER TABLE recordings DROP CONSTRAINT` + `ADD CONSTRAINT`.
- **MaxBytesReader ordering issue**: MaxBytesReader is set before media type is known. Use video limit (larger) for the reader, verify actual type after.
- **playback.m4a hardcoded**: Found in `processing.go:112`, stream/segment handlers, share handlers.
- **`processing_step` constraint name**: Need to check actual constraint name in DB to drop it. Migration 002 adds it as inline CHECK without explicit name — PostgreSQL auto-names it.

### Constraint Naming
- When using `ALTER TABLE recordings ADD COLUMN processing_step TEXT NOT NULL DEFAULT 'queued' CHECK (...)`, PostgreSQL generates a constraint name like `recordings_processing_step_check`. This needs to be dropped and re-created in migration 004.
