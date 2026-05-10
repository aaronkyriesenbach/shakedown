# Video Support — Implementation Plan

## TL;DR

> **Quick Summary**: Add video as a first-class recording type alongside audio in Shakedown. Videos are transcoded to H.264/AAC MP4 (capped at 1080p), thumbnails are extracted, and a native `<video>` player with custom Tailwind controls renders them in the unified library and share pages.
>
> **Deliverables**:
> - DB migration adding `media_type`, `thumbnail_ready`, `video_width`, `video_height` columns
> - Backend: video-aware magic byte validation, bifurcated processing pipeline, separate audio/video worker pools, updated stream/segment/share/thumbnail handlers
> - Frontend: `VideoPlayer` component with custom controls, conditional rendering in RecordingDetail/SharePage, thumbnail cards in library, video-enabled upload form
> - Go backend tests for new video handlers
>
> **Estimated Effort**: Large
> **Parallel Execution**: YES — 4 waves
> **Critical Path**: T1 (migration) → T6 (repo) → T7 (processing) → T14 (thumbnail endpoint) → T17 (RecordingDetail) → F1-F4

---

## Context

### Original Request
Add video support as a first-class recording type, peer to audio, based on design decisions documented in `docs/plans/2025-05-10-video-support-design.md`.

### Interview Summary
**Key Discussions**:
- **Waveform for video**: NO — Song markers will be CSS position overlays on the video scrubber bar
- **Video player**: Native `<video>` element with custom Tailwind controls, following the existing `WaveformPlayer`/`AudioControls` pattern
- **Transcode resolution**: Cap at 1080p (downscale anything above)
- **Thumbnail timing**: Extract frame at 10% of video duration
- **Thumbnail dimensions**: Scale to 640px wide, JPEG quality 80-85
- **Worker pool**: Separate audio and video pools to prevent contention
- **Test strategy**: Go backend tests only (no frontend tests exist in codebase)

**Research Findings**:
- Frontend stack: React 19 + Radix UI + Tailwind CSS + Vite + wavesurfer.js 7.x + Uppy
- Backend stack: Go + chi router + PostgreSQL (pgx) + ffmpeg/ffprobe/audiowaveform
- Current worker pool: single semaphore, 4 default workers, 300s timeout
- Upload limit: 500MB (insufficient for video)
- Shares handler hardcodes `playback.m4a`, `snippet.m4a`, `audio/mp4` throughout
- `ExtractSegment` function hardcodes audio-only ffmpeg flags
- Migrations: 001-003 exist, next is 004

### Metis Review
**Identified Gaps** (addressed):
- Share handler and ExtractSegment hardcode audio paths — branching on media_type required
- M4A vs MP4/MOV ftyp brand disambiguation: current prefix matching is insufficient, must parse brand field at bytes 8-11
- No-audio video edge case: screen recordings may lack audio track, processing must handle gracefully
- Short video edge case: 10% of <1s video rounds to 0s thumbnail, must clamp to minimum 0.1s
- Reverse proxy upload limits: nginx/caddy default body size limits may block 4GB uploads — document in deployment notes
- Playback filename helper: centralize `playback.m4a` vs `playback.mp4` resolution to avoid scattered conditionals

---

## Work Objectives

### Core Objective
Enable uploading, transcoding, storing, and playing back video recordings alongside existing audio recordings, with a unified library view, thumbnail-based cards for video, and full song/timestamp support.

### Concrete Deliverables
- `internal/database/migrations/004_video_support.{up,down}.sql`
- Updated `validation.go` with `ValidateMediaMagicBytes`
- Bifurcated `processRecording` in `processing.go` (audio path unchanged, video path: H.264 transcode + thumbnail)
- Separate worker pools in `service.go`
- Updated handlers: `streamRecording`, `segmentRecording`, `thumbnailRecording` (new), share handlers
- Updated repository queries for new columns
- Updated config with per-media-type settings
- `frontend/src/components/video/VideoPlayer.tsx` + `useVideoPlayer` hook
- Updated `RecordingDetail.tsx`, `SharePage.tsx`, `RecordingCard.tsx`, `UploadForm.tsx`
- Updated TypeScript types in `recordings.ts`, `shares.ts`
- Go backend tests in `handler_test.go`

### Definition of Done
- [ ] Video upload (MP4, MOV) accepted and transcoded to H.264/AAC MP4
- [ ] Thumbnail extracted and served at `/{recordingID}/thumbnail`
- [ ] Video plays in RecordingDetail with custom controls
- [ ] Video shares render with video player
- [ ] Library cards show thumbnail for video, music icon for audio
- [ ] Audio recordings completely unchanged in behavior
- [ ] `go test ./...` passes
- [ ] `tsc -b && vite build` succeeds

### Must Have
- Video as peer to audio — same upload flow, same library, same share system
- H.264/AAC MP4 output with faststart
- Resolution capped at 1080p
- Thumbnail at 10% duration, 640px wide, JPEG q80-85
- Separate audio/video worker pools
- Song markers supported for video (CSS overlays)
- Existing audio behavior completely untouched

### Must NOT Have (Guardrails)
- NO adaptive streaming (HLS/DASH)
- NO sprite sheets or hover previews
- NO separate upload flow or library view for video
- NO waveform generation for video recordings
- NO third-party video player library (use native `<video>`)
- NO changes to existing migration files (001-003)
- NO `any` types in TypeScript
- NO hardcoded `playback.m4a` strings remaining after implementation (must use media-type-aware helper)
- NO audio processing behavior changes — audio path must remain byte-for-byte identical

---

## Verification Strategy

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed. No exceptions.

### Test Decision
- **Infrastructure exists**: YES (Go tests in `internal/*/handler_test.go`)
- **Automated tests**: Tests-after (add tests for new video handlers)
- **Framework**: Go `testing` package (existing pattern)

### QA Policy
Every task MUST include agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **Backend API**: Use Bash (curl) — send requests, assert status + response fields
- **Frontend/UI**: Use Playwright (playwright skill) — navigate, interact, assert DOM, screenshot
- **Processing**: Use Bash — upload test video, poll until complete, verify artifacts exist
- **TUI/CLI**: Use interactive_bash (tmux) — run commands, validate output

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Foundation — all independent, start immediately):
├── T1:  DB migration 004                                    [quick]
├── T2:  Config updates (per-media timeout, upload, workers)  [quick]
├── T3:  Go struct updates (Recording, ProcessingJob, ffprobe) [quick]
├── T4:  Frontend TypeScript types + URL helpers               [quick]
└── T5:  Magic byte validation rewrite                         [quick]

Wave 2 (Core Backend + Frontend Components — depends on Wave 1):
├── T6:  Repository query updates (depends: T1, T3)            [unspecified-high]
├── T7:  Processing pipeline bifurcation (depends: T3)         [deep]
├── T8:  Worker pool separation (depends: T2, T3)              [quick]
├── T9:  Upload handler (depends: T3, T5)                      [unspecified-high]
├── T10: VideoPlayer component + hook (depends: T4)            [visual-engineering]
├── T11: Upload form update (depends: T4)                      [quick]
└── T12: RecordingCard thumbnails + badge (depends: T4)        [visual-engineering]

Wave 3 (Handlers + Frontend Integration — depends on Wave 2):
├── T13: Stream handler + thumbnail endpoint (depends: T6)     [unspecified-high]
├── T14: Segment handler video support (depends: T6)           [unspecified-high]
├── T15: Share handler + ExtractSegment (depends: T6, T7)      [unspecified-high]
├── T16: RecordingDetail conditional player (depends: T10)     [visual-engineering]
├── T17: SharePage video support (depends: T10)                [visual-engineering]
└── T18: Song markers CSS overlays (depends: T10)              [visual-engineering]

Wave 4 (Tests — depends on Wave 3):
└── T19: Backend Go tests (depends: T9, T13-T15)              [unspecified-high]

Wave FINAL (Verification — after ALL tasks):
├── F1: Plan compliance audit                                  [oracle]
├── F2: Code quality review                                    [unspecified-high]
├── F3: Real manual QA                                         [unspecified-high]
└── F4: Scope fidelity check                                   [deep]
→ Present results → Get explicit user okay
```

### Dependency Matrix

| Task | Depends On | Blocks | Wave |
|------|-----------|--------|------|
| T1   | —         | T6     | 1    |
| T2   | —         | T8     | 1    |
| T3   | —         | T6,T7,T8,T9 | 1 |
| T4   | —         | T10,T11,T12 | 1 |
| T5   | —         | T9     | 1    |
| T6   | T1,T3     | T13,T14,T15 | 2 |
| T7   | T3        | T15    | 2    |
| T8   | T2,T3     | —      | 2    |
| T9   | T3,T5     | T19    | 2    |
| T10  | T4        | T16,T17,T18 | 2 |
| T11  | T4        | —      | 2    |
| T12  | T4        | —      | 2    |
| T13  | T6        | T19    | 3    |
| T14  | T6        | T19    | 3    |
| T15  | T6,T7     | T19    | 3    |
| T16  | T10       | —      | 3    |
| T17  | T10       | —      | 3    |
| T18  | T10       | —      | 3    |
| T19  | T9,T13-T15| —      | 4    |

### Agent Dispatch Summary

- **Wave 1**: 5 tasks — T1-T5 → `quick`
- **Wave 2**: 7 tasks — T6 → `unspecified-high`, T7 → `deep`, T8 → `quick`, T9 → `unspecified-high`, T10 → `visual-engineering`, T11 → `quick`, T12 → `visual-engineering`
- **Wave 3**: 6 tasks — T13-T15 → `unspecified-high`, T16-T18 → `visual-engineering`
- **Wave 4**: 1 task — T19 → `unspecified-high`
- **FINAL**: 4 tasks — F1 → `oracle`, F2 → `unspecified-high`, F3 → `unspecified-high`, F4 → `deep`

---

## TODOs

- [ ] 1. DB Migration 004 — Video Support Schema

  **What to do**:
  - Create `internal/database/migrations/004_video_support.up.sql`:
    - `ALTER TABLE recordings ADD COLUMN media_type TEXT NOT NULL DEFAULT 'audio' CHECK (media_type IN ('audio','video'));`
    - `ALTER TABLE recordings ADD COLUMN thumbnail_ready BOOLEAN NOT NULL DEFAULT false;`
    - `ALTER TABLE recordings ADD COLUMN video_width INTEGER;`
    - `ALTER TABLE recordings ADD COLUMN video_height INTEGER;`
    - Drop the existing `processing_step` CHECK constraint from migration 002, re-add with expanded values: `('queued','analyzing','transcoding','extracting_thumbnail','generating_waveform','complete')`
  - Create `internal/database/migrations/004_video_support.down.sql`:
    - Drop columns `media_type`, `thumbnail_ready`, `video_width`, `video_height`
    - Restore original `processing_step` CHECK constraint

  **Must NOT do**:
  - Do NOT modify migration files 001, 002, or 003
  - Do NOT add columns that aren't needed (no `video_codec`, `video_fps` etc.)

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T2, T3, T4, T5)
  - **Blocks**: T6
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `internal/database/migrations/002_processing_step.up.sql` — Pattern for ALTER TABLE with CHECK constraints. Shows the exact constraint syntax used.
  - `internal/database/migrations/001_initial.up.sql:14-35` — Original recordings table schema. Shows all existing columns and their types/defaults.

  **Acceptance Criteria**:

  **QA Scenarios**:

  ```
  Scenario: Migration applies cleanly to existing database
    Tool: Bash
    Preconditions: Database has migrations 001-003 applied, contains existing audio recordings
    Steps:
      1. Run: migrate -path internal/database/migrations -database "$DATABASE_URL" up
      2. Assert: exit code 0, no errors
      3. Run: psql "$DATABASE_URL" -c "SELECT column_name, data_type, column_default FROM information_schema.columns WHERE table_name='recordings' AND column_name IN ('media_type','thumbnail_ready','video_width','video_height') ORDER BY column_name;"
      4. Assert: all 4 columns exist with correct types and defaults
      5. Run: psql "$DATABASE_URL" -c "SELECT media_type, thumbnail_ready FROM recordings LIMIT 5;"
      6. Assert: existing rows have media_type='audio', thumbnail_ready=false
    Expected Result: Migration applies without errors, existing data preserved with correct defaults
    Failure Indicators: Migration error, missing columns, incorrect defaults on existing rows
    Evidence: .sisyphus/evidence/task-1-migration-apply.txt

  Scenario: Down migration reverses cleanly
    Tool: Bash
    Preconditions: Migration 004 applied
    Steps:
      1. Run: migrate -path internal/database/migrations -database "$DATABASE_URL" down 1
      2. Assert: exit code 0
      3. Run: psql "$DATABASE_URL" -c "SELECT column_name FROM information_schema.columns WHERE table_name='recordings' AND column_name='media_type';"
      4. Assert: no rows returned (column removed)
    Expected Result: Down migration removes all video columns cleanly
    Failure Indicators: Error during rollback, columns still present
    Evidence: .sisyphus/evidence/task-1-migration-down.txt

  Scenario: Processing step CHECK accepts new values
    Tool: Bash
    Preconditions: Migration 004 applied
    Steps:
      1. Run: psql "$DATABASE_URL" -c "UPDATE recordings SET processing_step='extracting_thumbnail' WHERE id=(SELECT id FROM recordings LIMIT 1);"
      2. Assert: UPDATE 1 (no CHECK violation)
      3. Run: psql "$DATABASE_URL" -c "UPDATE recordings SET processing_step='invalid_step' WHERE id=(SELECT id FROM recordings LIMIT 1);"
      4. Assert: CHECK constraint violation error
    Expected Result: New step values accepted, invalid values rejected
    Evidence: .sisyphus/evidence/task-1-check-constraint.txt
  ```

  **Commit**: YES
  - Message: `feat(db): add video support migration 004`
  - Files: `internal/database/migrations/004_video_support.up.sql`, `internal/database/migrations/004_video_support.down.sql`

- [ ] 2. Config Updates — Per-Media Timeout, Upload Limit, Worker Counts

  **What to do**:
  - In `internal/config/config.go`, add new config fields:
    - `VideoProcessingTimeoutSeconds int` (default: 3600 — 1 hour) — separate from audio's 300s
    - `VideoUploadMaxSizeMB int64` (default: 4096 — 4GB) — separate from audio's 500MB
    - `VideoProcessingMaxWorkers int` (default: 2) — separate video worker pool
  - Rename existing `ProcessingMaxWorkers` semantically to represent audio workers (keep env var name `PROCESSING_MAX_WORKERS` for backward compat)
  - Add env var tags: `VIDEO_PROCESSING_TIMEOUT_SECONDS`, `VIDEO_UPLOAD_MAX_SIZE_MB`, `VIDEO_PROCESSING_MAX_WORKERS`

  **Must NOT do**:
  - Do NOT change the default values of existing audio config fields
  - Do NOT remove backward compatibility for existing env vars

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1, T3, T4, T5)
  - **Blocks**: T8
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `internal/config/config.go:10-29` — Existing Config struct with envconfig tags and defaults. Follow exact same pattern for new fields.

  **Acceptance Criteria**:

  **QA Scenarios**:

  ```
  Scenario: New config fields load with defaults
    Tool: Bash
    Preconditions: No VIDEO_* env vars set
    Steps:
      1. Build and run the app with only required env vars (DATABASE_URL, SESSION_SECRET, DISABLE_AUTH=true)
      2. Add a temporary log line or test that prints config values
      3. Assert: VideoProcessingTimeoutSeconds=3600, VideoUploadMaxSizeMB=4096, VideoProcessingMaxWorkers=2
      4. Assert: existing ProcessingMaxWorkers=4, ProcessingTimeoutSeconds=300, UploadMaxSizeMB=500 unchanged
    Expected Result: New defaults work, existing defaults unchanged
    Evidence: .sisyphus/evidence/task-2-config-defaults.txt

  Scenario: New config fields override via env vars
    Tool: Bash
    Steps:
      1. Set VIDEO_PROCESSING_TIMEOUT_SECONDS=7200, VIDEO_UPLOAD_MAX_SIZE_MB=8192, VIDEO_PROCESSING_MAX_WORKERS=4
      2. Load config and verify new values applied
    Expected Result: Overridden values take effect
    Evidence: .sisyphus/evidence/task-2-config-override.txt
  ```

  **Commit**: YES (groups with T3)
  - Message: `feat(recordings): add video fields to structs and config`
  - Files: `internal/config/config.go`

- [ ] 3. Go Struct Updates — Recording, ProcessingJob, ffprobeResult

  **What to do**:
  - In `internal/recordings/repository.go`, add to `Recording` struct:
    - `MediaType string` (json:"media_type")
    - `ThumbnailReady bool` (json:"thumbnail_ready")
    - `VideoWidth *int` (json:"video_width,omitempty")
    - `VideoHeight *int` (json:"video_height,omitempty")
  - In `internal/recordings/repository.go`, add to `CreateRecordingInput`:
    - `MediaType string`
  - In `internal/recordings/processing.go`, add to `ProcessingJob`:
    - `MediaType string`
  - In `internal/recordings/processing.go`, extend `ffprobeResult` streams to capture video fields:
    - Add `Width int`, `Height int`, `CodecName string` to the stream struct
  - Add a helper function `PlaybackFilename(mediaType string) string` that returns `"playback.m4a"` for audio and `"playback.mp4"` for video. Use this helper everywhere instead of hardcoded strings. Place in `processing.go` or a new `media.go` file.

  **Must NOT do**:
  - Do NOT modify the query strings yet (that's T6)
  - Do NOT change existing field types or tags
  - Do NOT add fields not specified in the migration (T1)

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1, T2, T4, T5)
  - **Blocks**: T6, T7, T8, T9
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `internal/recordings/repository.go:14-36` — Existing Recording struct. Follow the exact json tag and pointer-for-nullable pattern.
  - `internal/recordings/repository.go:38-48` — Existing CreateRecordingInput struct.
  - `internal/recordings/processing.go:99-104` — Existing ProcessingJob struct.
  - `internal/recordings/processing.go:15-27` — Existing ffprobeResult struct. The Streams slice has CodecType, SampleRate, Channels. Video fields (Width, Height, CodecName) use the same json tags as ffprobe JSON output.

  **Acceptance Criteria**:

  **QA Scenarios**:

  ```
  Scenario: Go code compiles with new struct fields
    Tool: Bash
    Steps:
      1. Run: go build ./internal/recordings/...
      2. Assert: exit code 0, no compilation errors
    Expected Result: All struct changes compile cleanly
    Evidence: .sisyphus/evidence/task-3-go-build.txt

  Scenario: PlaybackFilename helper returns correct values
    Tool: Bash
    Steps:
      1. Write a small test or use go run to verify PlaybackFilename("audio") == "playback.m4a"
      2. Verify PlaybackFilename("video") == "playback.mp4"
    Expected Result: Helper returns correct filenames for each media type
    Evidence: .sisyphus/evidence/task-3-playback-filename.txt
  ```

  **Commit**: YES (groups with T2)
  - Message: `feat(recordings): add video fields to structs and config`
  - Files: `internal/recordings/repository.go`, `internal/recordings/processing.go`

- [ ] 4. Frontend TypeScript Types + URL Helpers

  **What to do**:
  - In `frontend/src/api/recordings.ts`:
    - Add `media_type: 'audio' | 'video'` to `Recording` interface
    - Add `thumbnail_ready: boolean` to `Recording` interface
    - Add `video_width?: number` and `video_height?: number` to `Recording` interface
    - Add `'extracting_thumbnail'` to the `processing_step` union type
    - Add `media_type` to `CreateRecordingInput` interface
    - Add `thumbnailUrl(id: string): string` function returning `/api/recordings/${id}/thumbnail`
  - In `frontend/src/api/shares.ts`:
    - Add `shareStreamUrl` and `shareDownloadUrl` awareness — these don't need to change (backend handles media type), but add a `shareThumbnailUrl(token: string): string` if the share API exposes thumbnails

  **Must NOT do**:
  - Do NOT add `any` types
  - Do NOT change existing field types (only add new ones)
  - Do NOT remove the `waveform_ready` field (still used for audio)

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1, T2, T3, T5)
  - **Blocks**: T10, T11, T12
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `frontend/src/api/recordings.ts:4-25` — Existing Recording interface with all current fields and the processing_step union type
  - `frontend/src/api/recordings.ts:128-143` — Existing URL helper functions (streamUrl, downloadUrl, waveformUrl, segmentUrl). Follow same pattern.
  - `frontend/src/api/shares.ts:55-65` — Existing share URL helpers

  **Acceptance Criteria**:

  **QA Scenarios**:

  ```
  Scenario: TypeScript compiles with new types
    Tool: Bash
    Preconditions: frontend directory
    Steps:
      1. Run: cd frontend && npx tsc -b --noEmit
      2. Assert: exit code 0, no type errors
    Expected Result: All type additions compile cleanly
    Evidence: .sisyphus/evidence/task-4-tsc.txt

  Scenario: Existing code still type-checks
    Tool: Bash
    Steps:
      1. Run: cd frontend && npx tsc -b --noEmit
      2. Assert: no new errors introduced (new optional fields don't break existing consumers)
    Expected Result: Zero regressions
    Evidence: .sisyphus/evidence/task-4-no-regression.txt
  ```

  **Commit**: YES (groups with T11)
  - Message: `feat(frontend): update types and upload form for video`
  - Files: `frontend/src/api/recordings.ts`, `frontend/src/api/shares.ts`

- [ ] 5. Magic Byte Validation — Extend for Video Formats

  **What to do**:
  - Rename `ValidateAudioMagicBytes` to `ValidateMediaMagicBytes` in `internal/recordings/validation.go`
  - Replace the simple prefix-matching approach with proper ftyp brand parsing for ISO base media format files:
    - Read first 32 bytes (already done)
    - For ftyp-based files: bytes 4-7 are always `ftyp`. Bytes 8-11 contain the major brand:
      - `M4A ` (0x4D344120) → `audio/mp4`, `.m4a` (existing audio)
      - `isom`, `mp42`, `mp41`, `avc1` → `video/mp4`, `.mp4` (video)
      - `qt  ` (0x71742020) → `video/quicktime`, `.mov` (video)
    - The first 4 bytes are the box size (variable) — do NOT hardcode. Instead, search for `ftyp` at offset 4 in the header.
  - Keep all existing audio signatures (MP3, FLAC, WAV, OGG) unchanged
  - Return a `mediaType` field (not just MIME) so callers know if it's audio or video. Either return an additional string or infer from MIME prefix.
  - Update the caller in `handler.go:80` — the function name changes but this is T9's job. For now, just ensure the old function name is removed and the new one exported.

  **Must NOT do**:
  - Do NOT use ffprobe for format detection (magic bytes only)
  - Do NOT break existing audio format detection
  - Do NOT accept formats beyond MP4, MOV, M4A for ISO base media (no MKV, AVI, etc.)

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1, T2, T3, T4)
  - **Blocks**: T9
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `internal/recordings/validation.go:1-45` — Current validation implementation. Shows the prefix-matching pattern and how the validated reader is returned.

  **External References**:
  - ISO base media file format: ftyp box structure — bytes 0-3: box size, 4-7: `ftyp`, 8-11: major brand, 12-15: minor version, 16+: compatible brands

  **Acceptance Criteria**:

  **QA Scenarios**:

  ```
  Scenario: MP4 video file detected correctly
    Tool: Bash
    Steps:
      1. Create a test with a byte buffer starting with ftyp box: size(4) + "ftyp"(4) + "isom"(4) + padding
      2. Call ValidateMediaMagicBytes
      3. Assert: returns "video/mp4", ".mp4"
    Expected Result: MP4 video correctly identified by brand
    Evidence: .sisyphus/evidence/task-5-mp4-detection.txt

  Scenario: MOV file detected correctly
    Tool: Bash
    Steps:
      1. Create test with ftyp box: size + "ftyp" + "qt  " + padding
      2. Call ValidateMediaMagicBytes
      3. Assert: returns "video/quicktime", ".mov"
    Expected Result: MOV correctly identified by qt brand
    Evidence: .sisyphus/evidence/task-5-mov-detection.txt

  Scenario: M4A audio still detected correctly (no regression)
    Tool: Bash
    Steps:
      1. Create test with ftyp box: size + "ftyp" + "M4A " + padding
      2. Call ValidateMediaMagicBytes
      3. Assert: returns "audio/mp4", ".m4a"
    Expected Result: M4A audio still works after refactor
    Evidence: .sisyphus/evidence/task-5-m4a-regression.txt

  Scenario: MP3/FLAC/WAV/OGG still detected (no regression)
    Tool: Bash
    Steps:
      1. Test each audio format with its known magic bytes
      2. Assert: all return correct MIME type and extension
    Expected Result: All existing audio formats unaffected
    Evidence: .sisyphus/evidence/task-5-audio-regression.txt

  Scenario: Unknown format rejected
    Tool: Bash
    Steps:
      1. Create test with random bytes that don't match any signature
      2. Call ValidateMediaMagicBytes
      3. Assert: returns error
    Expected Result: Unknown formats rejected with clear error
    Evidence: .sisyphus/evidence/task-5-unknown-reject.txt
  ```

  **Commit**: YES
  - Message: `feat(recordings): extend magic byte validation for video formats`
  - Files: `internal/recordings/validation.go`

- [ ] 6. Repository Query Updates — New Columns in All Queries

  **What to do**:
  - Update ALL SQL queries in `internal/recordings/repository.go` to include the new columns:
    - `Create` / `createWithAutoTitle` INSERT: add `media_type` to the INSERT column list and values. Add `media_type`, `thumbnail_ready`, `video_width`, `video_height` to RETURNING clause. Add corresponding `.Scan()` fields.
    - `GetByID` SELECT: add `media_type`, `thumbnail_ready`, `video_width`, `video_height` to SELECT list and `.Scan()`.
    - `List` (if exists): same as GetByID
    - `UpdateProcessingResult`: add `thumbnail_ready`, `video_width`, `video_height` parameters. Update the UPDATE SET clause and function signature.
    - `UpdateProcessingStep`: no changes needed (just updates step string)
  - Ensure the `Scan` call order matches the SELECT column order exactly for each query

  **Must NOT do**:
  - Do NOT change query logic or add new queries (thumbnail endpoint query is T13)
  - Do NOT add `any`-typed parameters to functions
  - Do NOT change the function signatures of methods not touched by video (e.g., SoftDelete)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with T7, T8, T9, T10, T11, T12)
  - **Blocks**: T13, T14, T15
  - **Blocked By**: T1, T3

  **References**:

  **Pattern References**:
  - `internal/recordings/repository.go:70-93` — `Create` method: Shows INSERT + RETURNING + Scan pattern. New columns follow this pattern.
  - `internal/recordings/repository.go:97-144` — `createWithAutoTitle`: Same INSERT pattern inside a transaction. Must add same columns.
  - `internal/recordings/repository.go:146-171` — `GetByID`: SELECT + Scan pattern. Add new columns in same position.
  - `internal/recordings/repository.go:184-215` — `UpdateProcessingResult`: UPDATE SET + function params. Must add new video metadata params.

  **Acceptance Criteria**:

  **QA Scenarios**:

  ```
  Scenario: Create recording with media_type='video' persists correctly
    Tool: Bash
    Steps:
      1. Call repo.Create with MediaType="video" and verify returned Recording has MediaType="video"
      2. Call repo.GetByID and verify MediaType="video" is returned
    Expected Result: Video media type round-trips through create and read
    Evidence: .sisyphus/evidence/task-6-create-video.txt

  Scenario: UpdateProcessingResult stores video metadata
    Tool: Bash
    Steps:
      1. Create a recording with media_type='video'
      2. Call UpdateProcessingResult with thumbnailReady=true, videoWidth=1920, videoHeight=1080
      3. Call GetByID and verify all video fields are set
    Expected Result: Video metadata persisted and readable
    Evidence: .sisyphus/evidence/task-6-update-video-meta.txt

  Scenario: Existing audio queries still work (no regression)
    Tool: Bash
    Steps:
      1. Create a recording with default (audio) media type
      2. Call GetByID and verify media_type='audio', thumbnail_ready=false, video_width=nil, video_height=nil
    Expected Result: Audio recordings unaffected
    Evidence: .sisyphus/evidence/task-6-audio-regression.txt
  ```

  **Commit**: YES
  - Message: `feat(recordings): update repository queries for video columns`
  - Files: `internal/recordings/repository.go`

- [ ] 7. Processing Pipeline Bifurcation — Video Transcode + Thumbnail

  **What to do**:
  - In `internal/recordings/processing.go`, modify `processRecording` to branch on `job.MediaType`:
  - **Audio path** (media_type == "audio"): Completely unchanged. Same ffprobe → ffmpeg AAC → audiowaveform flow.
  - **Video path** (media_type == "video"):
    1. `ffprobe`: Parse BOTH audio and video streams from `probeResult.Streams`. Extract video stream fields: Width, Height, CodecName. Audio streams: same as before (sample_rate, channels). Handle edge case: video with no audio track (set sample_rate=0, channels=0).
    2. `ffmpeg` transcode: Build command dynamically:
       - Base: `-c:v libx264 -c:a aac -movflags +faststart`
       - If source height > 1080: add `-vf scale=-2:1080` to downscale (use -2 for even width)
       - If source is already H.264 at ≤1080p: use `-c:v copy` instead of re-encoding (fast path)
       - Output: `playback.mp4` (use `PlaybackFilename("video")` helper from T3)
       - Add `-y` flag for overwrite
    3. Thumbnail extraction: `ffmpeg -ss {10% of duration} -i {original} -vframes 1 -vf scale=640:-2 -q:v 2 -y thumbnail.jpg`
       - Edge case: if duration < 1s, clamp seek to 0.1s
       - Update processing step to `extracting_thumbnail` before this step
    4. Set `thumbnailReady = true` after successful extraction
    5. Skip audiowaveform step entirely for video (waveformReady stays false)
    6. Call `UpdateProcessingResult` with video-specific metadata (thumbnailReady, videoWidth, videoHeight)
  - Add a `runFFmpegVideo` function (or parameterize `runFFmpeg`) for the video transcode command
  - Add a `extractThumbnail` function for the thumbnail step

  **Must NOT do**:
  - Do NOT modify the audio path in any way
  - Do NOT generate waveforms for video recordings
  - Do NOT use adaptive bitrate or multiple output resolutions
  - Do NOT add HLS/DASH output

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with T6, T8, T9, T10, T11, T12)
  - **Blocks**: T15
  - **Blocked By**: T3

  **References**:

  **Pattern References**:
  - `internal/recordings/processing.go:109-175` — Current `processRecording` function. This is the function to bifurcate. Shows the full ffprobe→ffmpeg→audiowaveform flow.
  - `internal/recordings/processing.go:50-65` — `runFFmpeg` function. Audio-only AAC transcode. Video path needs a different command.
  - `internal/recordings/processing.go:30-48` — `runFFprobe` function. Already captures all streams, but the result struct only has audio fields. T3 adds video fields.
  - `internal/recordings/processing.go:67-97` — `runAudiowaveform` function. Video path should skip this entirely.

  **External References**:
  - ffmpeg H.264 encoding: `-c:v libx264 -preset medium -crf 23` (or `-c:v copy` for passthrough)
  - ffmpeg thumbnail: `-vframes 1 -vf scale=640:-2 -q:v 2` (scale width to 640, auto height, JPEG quality ~85)
  - ffmpeg H.265 decode: modern ffmpeg includes HEVC decoder by default. Verify with `ffmpeg -decoders | grep hevc`
  - ffmpeg autorotate: on by default since ffmpeg 3.3. Do NOT add `-noautorotate` flag.

  **Acceptance Criteria**:

  **QA Scenarios**:

  ```
  Scenario: MP4 video processed to H.264 playback file
    Tool: Bash
    Preconditions: A test MP4 video file exists
    Steps:
      1. Upload the MP4 via the upload endpoint (after T9 is complete for integration test, or test processRecording directly)
      2. Wait for processing to complete (poll processing_step)
      3. Assert: file exists at {storageRoot}/{id}/playback.mp4
      4. Run: ffprobe -v quiet -print_format json -show_streams {storageRoot}/{id}/playback.mp4
      5. Assert: video codec is h264, audio codec is aac
      6. Assert: if source was >1080p, output height is 1080
    Expected Result: Video transcoded to H.264/AAC MP4 with faststart
    Evidence: .sisyphus/evidence/task-7-video-transcode.txt

  Scenario: Thumbnail extracted at correct position
    Tool: Bash
    Steps:
      1. After processing, assert file exists at {storageRoot}/{id}/thumbnail.jpg
      2. Run: ffprobe -v quiet -print_format json -show_streams thumbnail.jpg (or use `identify` if imagemagick available)
      3. Assert: width is 640px (or proportional), JPEG format
    Expected Result: Thumbnail exists, correct dimensions and format
    Evidence: .sisyphus/evidence/task-7-thumbnail-extract.jpg

  Scenario: Audio recording processing unchanged
    Tool: Bash
    Steps:
      1. Upload an audio file (MP3 or M4A)
      2. Wait for processing to complete
      3. Assert: playback.m4a exists (NOT playback.mp4)
      4. Assert: waveform.json exists
      5. Assert: thumbnail.jpg does NOT exist
      6. Assert: media_type='audio' in DB
    Expected Result: Audio path completely unchanged
    Evidence: .sisyphus/evidence/task-7-audio-unchanged.txt

  Scenario: Video with no audio track handled gracefully
    Tool: Bash
    Steps:
      1. Create/find a test video with no audio track (screen recording)
      2. Process it through the pipeline
      3. Assert: processing completes without error
      4. Assert: playback.mp4 exists (video-only)
    Expected Result: No-audio video processed without crash
    Evidence: .sisyphus/evidence/task-7-no-audio-video.txt
  ```

  **Commit**: YES (groups with T8)
  - Message: `feat(recordings): bifurcate processing pipeline and separate worker pools`
  - Files: `internal/recordings/processing.go`

- [ ] 8. Worker Pool Separation — Audio + Video Semaphores

  **What to do**:
  - In `internal/recordings/service.go`:
    - Replace single `sem chan struct{}` with two: `audioSem chan struct{}` and `videoSem chan struct{}`
    - Update `NewService` to accept both `maxAudioWorkers` and `maxVideoWorkers` (from config T2)
    - Update `Enqueue` to select the correct semaphore based on `job.MediaType`
    - Use the correct timeout: audio uses `ProcessingTimeoutSeconds`, video uses `VideoProcessingTimeoutSeconds`
  - In `cmd/server/main.go` (line 85, where `NewService` is called):
    - Pass both `cfg.ProcessingMaxWorkers` and `cfg.VideoProcessingMaxWorkers` to the updated `NewService`

  **Must NOT do**:
  - Do NOT change the overall goroutine-per-job architecture
  - Do NOT add priority queues or job scheduling (keep simple semaphore model)
  - Do NOT change audio worker defaults (4 workers, 300s timeout)

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with T6, T7, T9, T10, T11, T12)
  - **Blocks**: None
  - **Blocked By**: T2, T3

  **References**:

  **Pattern References**:
  - `internal/recordings/service.go:1-61` — Current Service struct with single semaphore, NewService, Enqueue, Shutdown. The semaphore pattern (`sem <- struct{}{}` to acquire, `<-svc.sem` to release) stays the same, just duplicated.
  - `internal/config/config.go` — Config struct where worker counts come from (after T2 adds video fields)

  **Acceptance Criteria**:

  **QA Scenarios**:

  ```
  Scenario: Audio and video jobs use separate pools
    Tool: Bash
    Steps:
      1. Run the app with PROCESSING_MAX_WORKERS=2 and VIDEO_PROCESSING_MAX_WORKERS=1
      2. Upload 3 audio files simultaneously → verify all 3 enqueue (2 run, 1 waits)
      3. Upload 1 video file → verify it runs on its own pool (not blocked by audio)
    Expected Result: Video job runs immediately despite audio pool being full
    Evidence: .sisyphus/evidence/task-8-separate-pools.txt

  Scenario: Video timeout is separate from audio
    Tool: Bash
    Steps:
      1. Set PROCESSING_TIMEOUT_SECONDS=10 and VIDEO_PROCESSING_TIMEOUT_SECONDS=3600
      2. Verify video processing gets 3600s timeout, audio gets 10s
    Expected Result: Each media type uses its own timeout
    Evidence: .sisyphus/evidence/task-8-separate-timeouts.txt
  ```

  **Commit**: YES (groups with T7)
  - Message: `feat(recordings): bifurcate processing pipeline and separate worker pools`
  - Files: `internal/recordings/service.go`

- [ ] 9. Upload Handler — Video Validation + MediaType

  **What to do**:
  - In `internal/recordings/handler.go`, update the `upload` function:
    - Line 80: Change `ValidateAudioMagicBytes(file)` to `ValidateMediaMagicBytes(file)` (renamed in T5)
    - Determine `mediaType` from the returned MIME type: if MIME starts with `video/` → "video", else → "audio"
    - Use the correct upload size limit: check `mediaType` and use `cfg.VideoUploadMaxSizeMB` for video, `cfg.UploadMaxSizeMB` for audio
    - Note: the `MaxBytesReader` is set at line 65 BEFORE we know the media type. Options: (a) use the larger of the two limits for MaxBytesReader, then validate actual size after classification, or (b) set MaxBytesReader to the video limit and let the pipeline handle it
    - Pass `MediaType` to `CreateRecordingInput` (line 126-135)
    - Pass `MediaType` to `ProcessingJob` (line 162-165)
    - Use the correct timeout for `Enqueue`: `cfg.ProcessingTimeoutSeconds` for audio, `cfg.VideoProcessingTimeoutSeconds` for video

  **Must NOT do**:
  - Do NOT change the Uppy upload flow structure
  - Do NOT add separate upload endpoints for video
  - Do NOT change the response format

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with T6, T7, T8, T10, T11, T12)
  - **Blocks**: T19
  - **Blocked By**: T3, T5

  **References**:

  **Pattern References**:
  - `internal/recordings/handler.go:58-170` — Current `upload` function. Line 80 calls ValidateAudioMagicBytes. Line 65 sets MaxBytesReader. Lines 126-135 create the recording. Lines 162-165 enqueue the processing job.

  **Acceptance Criteria**:

  **QA Scenarios**:

  ```
  Scenario: Video upload accepted and enqueued with correct media type
    Tool: Bash
    Steps:
      1. curl -X POST -F "file=@test.mp4" http://localhost:8080/api/recordings (with auth)
      2. Assert: HTTP 201
      3. Assert: response JSON has media_type: "video"
      4. Poll processing_step until complete
      5. Assert: recording processed as video (playback.mp4 exists)
    Expected Result: Video upload goes through the video processing path
    Evidence: .sisyphus/evidence/task-9-video-upload.txt

  Scenario: Audio upload still works (regression test)
    Tool: Bash
    Steps:
      1. curl -X POST -F "file=@test.mp3" http://localhost:8080/api/recordings (with auth)
      2. Assert: HTTP 201
      3. Assert: response JSON has media_type: "audio"
    Expected Result: Audio upload path unchanged
    Evidence: .sisyphus/evidence/task-9-audio-regression.txt

  Scenario: Oversized video rejected
    Tool: Bash
    Preconditions: VIDEO_UPLOAD_MAX_SIZE_MB=1
    Steps:
      1. Attempt to upload a >1MB video file
      2. Assert: HTTP 413 or appropriate error
    Expected Result: Size limit enforced per media type
    Evidence: .sisyphus/evidence/task-9-size-limit.txt
  ```

  **Commit**: YES
  - Message: `feat(recordings): update upload handler for video support`
  - Files: `internal/recordings/handler.go`

- [ ] 10. VideoPlayer Component + useVideoPlayer Hook

  **What to do**:
  - Create `frontend/src/components/video/VideoPlayer.tsx`:
    - Follow the architecture of `WaveformPlayer.tsx` (forwardRef, imperative handle for seekTo)
    - Props: `recording: Recording`, `streamUrlOverride?: string`, `onTimeUpdate?: (time: number) => void`, `onSeek?: (time: number) => void`, `className?: string`
    - Render a native `<video>` element with `poster={thumbnailUrl(recording.id)}` (when `thumbnail_ready`)
    - Below the video: render custom `VideoControls` component
    - Show `ProcessingStatus` when `processing_step !== 'complete'` (reuse existing component)
    - Keyboard shortcuts: Space (play/pause), Left/Right arrows (±5s seek) — same pattern as WaveformPlayer
  - Create `frontend/src/hooks/useVideoPlayer.ts`:
    - Manages a `<video>` element ref
    - Exposes: `isPlaying`, `currentTime`, `duration`, `isReady`, `volume`, `togglePlay`, `seekToTime`, `setVolume`, `seek` (percentage-based)
    - Handles video events: `loadedmetadata`, `timeupdate`, `play`, `pause`, `ended`, `canplay`
  - Create `frontend/src/components/video/VideoControls.tsx`:
    - Reuse structure from `AudioControls` — play/pause button, time display, volume slider, seek bar
    - Add fullscreen button (video-specific)
    - The seek bar should be a simple progress bar / range input with Tailwind styling, not a waveform

  **Must NOT do**:
  - Do NOT install any third-party video player library
  - Do NOT duplicate AudioControls logic — extract shared utilities if >30% overlap
  - Do NOT add `any` types
  - Do NOT add waveform rendering for video

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with T6, T7, T8, T9, T11, T12)
  - **Blocks**: T16, T17, T18
  - **Blocked By**: T4

  **References**:

  **Pattern References**:
  - `frontend/src/components/audio/WaveformPlayer.tsx:1-107` — Full component: forwardRef pattern, imperative handle, conditional rendering on processing_step, container ref, keyboard shortcuts. Follow this structure closely.
  - `frontend/src/hooks/useAudioPlayer.ts` — Audio player hook. VideoPlayer hook follows same state management pattern (isPlaying, currentTime, etc.) but uses native `<video>` element instead of wavesurfer.
  - `frontend/src/components/audio/AudioControls.tsx` — Controls component with play/pause, volume, seek, time display. VideoControls should match visual style.
  - `frontend/src/components/audio/ProcessingStatus.tsx` — Reusable processing status component. Import and use directly.

  **API/Type References**:
  - `frontend/src/api/recordings.ts:4-25` — Recording type (after T4 adds media_type, thumbnail_ready)
  - `frontend/src/api/recordings.ts:128-143` — URL helpers including thumbnailUrl (after T4)

  **Acceptance Criteria**:

  **QA Scenarios**:

  ```
  Scenario: VideoPlayer renders and plays video
    Tool: Playwright
    Preconditions: A video recording exists and is fully processed
    Steps:
      1. Navigate to /recordings/{videoRecordingId}
      2. Wait for video element: document.querySelector('video')
      3. Assert: video element has src attribute pointing to stream URL
      4. Assert: video element has poster attribute (thumbnail URL)
      5. Click play button (or the play overlay)
      6. Wait 2 seconds
      7. Assert: video currentTime > 0
      8. Screenshot
    Expected Result: Video plays with poster thumbnail and custom controls
    Evidence: .sisyphus/evidence/task-10-video-plays.png

  Scenario: VideoPlayer keyboard controls work
    Tool: Playwright
    Steps:
      1. Navigate to video recording detail page
      2. Press Space → assert video plays
      3. Press Space again → assert video pauses
      4. Press ArrowRight → assert currentTime increased by ~5s
      5. Press ArrowLeft → assert currentTime decreased by ~5s
    Expected Result: Keyboard shortcuts match audio player behavior
    Evidence: .sisyphus/evidence/task-10-keyboard.png

  Scenario: VideoPlayer shows processing status when not complete
    Tool: Playwright
    Preconditions: A video recording exists with processing_step !== 'complete'
    Steps:
      1. Navigate to /recordings/{processingVideoId}
      2. Assert: ProcessingStatus component visible (text contains processing step name)
      3. Assert: video element NOT present
    Expected Result: Processing status shown instead of player
    Evidence: .sisyphus/evidence/task-10-processing-status.png

  Scenario: TypeScript compiles with new components
    Tool: Bash
    Steps:
      1. Run: cd frontend && npx tsc -b --noEmit
      2. Assert: exit code 0
    Expected Result: All new components type-check
    Evidence: .sisyphus/evidence/task-10-tsc.txt
  ```

  **Commit**: YES
  - Message: `feat(frontend): add VideoPlayer component with custom controls`
  - Files: `frontend/src/components/video/VideoPlayer.tsx`, `frontend/src/hooks/useVideoPlayer.ts`, `frontend/src/components/video/VideoControls.tsx`

- [ ] 11. Upload Form — Accept Video MIME Types

  **What to do**:
  - In `frontend/src/components/recordings/UploadForm.tsx`:
    - Line 51: Change Uppy restrictions from `allowedFileTypes: ['audio/*']` to `allowedFileTypes: ['audio/*', 'video/mp4', 'video/quicktime']`
    - Update the icon: when files include video types, show a `Video` icon (from lucide-react) alongside or instead of `Music`
    - Update the "Shared Audio" text in error/empty states to "Shared Recording" or similar media-agnostic language
    - The polling logic (lines 116-154) already checks `processing_step !== 'complete'` — verify this still works with new video processing steps (it should, since they're all intermediate states before 'complete')

  **Must NOT do**:
  - Do NOT create a separate upload form for video
  - Do NOT change the Uppy upload endpoint or multipart structure
  - Do NOT add file size validation on the frontend (backend handles it)

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with T6, T7, T8, T9, T10, T12)
  - **Blocks**: None
  - **Blocked By**: T4

  **References**:

  **Pattern References**:
  - `frontend/src/components/recordings/UploadForm.tsx:48-63` — Uppy initialization with restrictions. Change `allowedFileTypes` here.
  - `frontend/src/components/recordings/UploadForm.tsx:116-154` — Polling logic. Verify new processing steps (e.g., `extracting_thumbnail`) don't break the `!== 'complete'` check.

  **Acceptance Criteria**:

  **QA Scenarios**:

  ```
  Scenario: Video file accepted in upload form
    Tool: Playwright
    Steps:
      1. Navigate to upload page
      2. Attach a .mp4 file via file input
      3. Assert: file appears in the upload list (not rejected by Uppy restrictions)
      4. Click upload
      5. Assert: upload succeeds (no Uppy restriction error)
    Expected Result: MP4 and MOV files accepted in upload form
    Evidence: .sisyphus/evidence/task-11-video-upload-form.png

  Scenario: Audio file still accepted (regression)
    Tool: Playwright
    Steps:
      1. Attach a .mp3 file
      2. Assert: file accepted, upload works
    Expected Result: Audio upload unchanged
    Evidence: .sisyphus/evidence/task-11-audio-regression.png
  ```

  **Commit**: YES (groups with T4)
  - Message: `feat(frontend): update types and upload form for video`
  - Files: `frontend/src/components/recordings/UploadForm.tsx`

- [ ] 12. RecordingCard — Thumbnails + Media Type Badge

  **What to do**:
  - In `frontend/src/components/recordings/RecordingCard.tsx`:
    - Replace the static `Music` icon with conditional rendering:
      - If `recording.media_type === 'video'` AND `recording.thumbnail_ready`: show `<img src={thumbnailUrl(recording.id)} />` with aspect-ratio handling, loading state, and error fallback to `Video` icon
      - If `recording.media_type === 'video'` AND NOT `thumbnail_ready`: show `Video` icon (from lucide-react)
      - If `recording.media_type === 'audio'`: show `Music` icon (current behavior)
    - Add a small badge/icon overlay to distinguish media type at a glance (e.g., small `Video`/`Music` icon in corner of the card)
    - Handle thumbnail loading: use `loading="lazy"` on the img, add an `onError` handler to fall back to the icon
    - Keep the existing card layout structure — thumbnail replaces the 40x40 icon area, possibly expanding it slightly for video thumbnails

  **Must NOT do**:
  - Do NOT change the card layout structure for audio recordings
  - Do NOT add hover previews or animated thumbnails
  - Do NOT add sprite sheets

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with T6, T7, T8, T9, T10, T11)
  - **Blocks**: None
  - **Blocked By**: T4

  **References**:

  **Pattern References**:
  - `frontend/src/components/recordings/RecordingCard.tsx:1-85` — Full component. The icon area is lines 23-25 (40x40 div with Music icon). This is what becomes conditional.
  - `frontend/src/components/recordings/RecordingCard.tsx:46-61` — Processing status badge area. Media type badge follows same Badge pattern.

  **API/Type References**:
  - `frontend/src/api/recordings.ts` — Recording type with media_type, thumbnail_ready (after T4). thumbnailUrl helper.

  **Acceptance Criteria**:

  **QA Scenarios**:

  ```
  Scenario: Video recording card shows thumbnail
    Tool: Playwright
    Preconditions: A fully processed video recording exists
    Steps:
      1. Navigate to / (library page)
      2. Find the video recording card
      3. Assert: card contains an <img> element with src matching thumbnail URL
      4. Assert: card has a video badge/indicator
      5. Screenshot
    Expected Result: Video card shows thumbnail image and video indicator
    Evidence: .sisyphus/evidence/task-12-video-card.png

  Scenario: Audio recording card unchanged
    Tool: Playwright
    Steps:
      1. Navigate to / (library page)
      2. Find an audio recording card
      3. Assert: card shows Music icon (no thumbnail img)
      4. Assert: card has audio badge or no media type badge (existing behavior)
    Expected Result: Audio cards visually unchanged
    Evidence: .sisyphus/evidence/task-12-audio-card.png

  Scenario: Thumbnail loading error falls back to icon
    Tool: Playwright
    Steps:
      1. Find a video recording where thumbnail URL returns 404 (or mock it)
      2. Assert: card falls back to Video icon instead of broken image
    Expected Result: Graceful degradation on thumbnail error
    Evidence: .sisyphus/evidence/task-12-thumbnail-fallback.png
  ```

  **Commit**: YES (groups with T16, T17, T18)
  - Message: `feat(frontend): video rendering in cards, detail, share, and song markers`
  - Files: `frontend/src/components/recordings/RecordingCard.tsx`

- [ ] 13. Stream Handler + Thumbnail Endpoint

  **What to do**:
  - In `internal/recordings/handler.go`, update `streamRecording` (lines 271-301):
    - Replace hardcoded `"playback.m4a"` with `PlaybackFilename(rec.MediaType)` helper
    - Replace hardcoded `"audio/mp4"` Content-Type: use `"video/mp4"` for video, `"audio/mp4"` for audio (derive from `rec.MediaType`)
    - The `http.ServeContent` call already handles Range requests — no changes needed there
  - Add new `thumbnailRecording` handler:
    - `GET /{recordingID}/thumbnail`
    - Check `rec.ThumbnailReady` — if false, return 404
    - Serve `{storageRoot}/{id}/thumbnail.jpg` with `Content-Type: image/jpeg`
    - Add `Cache-Control: public, max-age=86400` header (thumbnails are immutable)
  - Register the new route in `Routes` (line 44-55): add `r.Get("/thumbnail", h.thumbnailRecording)` — no auth required (or same auth as stream)

  **Must NOT do**:
  - Do NOT change the Range request handling (http.ServeContent handles it)
  - Do NOT add thumbnail generation logic here (that's in processing, T7)
  - Do NOT serve thumbnails for audio recordings (return 404 if not thumbnail_ready)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with T14, T15, T16, T17, T18)
  - **Blocks**: T19
  - **Blocked By**: T6

  **References**:

  **Pattern References**:
  - `internal/recordings/handler.go:271-301` — Current `streamRecording`. Line 285: hardcoded `playback.m4a`. Line 299: hardcoded `audio/mp4`. These are the two lines to make media-type-aware.
  - `internal/recordings/handler.go:330-359` — `waveformData` handler. Follow same pattern for thumbnail (check ready flag, serve file, set content type).
  - `internal/recordings/handler.go:42-56` — Route registration. Add thumbnail route here.

  **Acceptance Criteria**:

  **QA Scenarios**:

  ```
  Scenario: Video stream returns video/mp4
    Tool: Bash
    Steps:
      1. curl -I http://localhost:8080/api/recordings/{videoId}/stream (with auth cookie)
      2. Assert: HTTP 200
      3. Assert: Content-Type is video/mp4
    Expected Result: Video recordings stream as video/mp4
    Evidence: .sisyphus/evidence/task-13-video-stream.txt

  Scenario: Audio stream still returns audio/mp4 (regression)
    Tool: Bash
    Steps:
      1. curl -I http://localhost:8080/api/recordings/{audioId}/stream (with auth cookie)
      2. Assert: HTTP 200
      3. Assert: Content-Type is audio/mp4
    Expected Result: Audio streaming unchanged
    Evidence: .sisyphus/evidence/task-13-audio-stream.txt

  Scenario: Thumbnail served for video recording
    Tool: Bash
    Steps:
      1. curl -I http://localhost:8080/api/recordings/{videoId}/thumbnail (with auth cookie)
      2. Assert: HTTP 200
      3. Assert: Content-Type is image/jpeg
      4. Assert: Cache-Control header present
    Expected Result: Thumbnail served correctly
    Evidence: .sisyphus/evidence/task-13-thumbnail.txt

  Scenario: Thumbnail 404 for audio recording
    Tool: Bash
    Steps:
      1. curl -I http://localhost:8080/api/recordings/{audioId}/thumbnail
      2. Assert: HTTP 404
    Expected Result: Audio recordings return 404 for thumbnail
    Evidence: .sisyphus/evidence/task-13-thumbnail-404.txt
  ```

  **Commit**: YES (groups with T14)
  - Message: `feat(recordings): add video stream, thumbnail, and segment handlers`
  - Files: `internal/recordings/handler.go`

- [ ] 14. Segment Handler — Video Support

  **What to do**:
  - In `internal/recordings/handler.go`, update `segmentRecording` (lines 361-408):
    - Line 390: Replace hardcoded `"playback.m4a"` with `PlaybackFilename(rec.MediaType)`
    - Lines 393-394: Branch on `rec.MediaType`:
      - Audio: keep current `audio/mp4` Content-Type, `segment.m4a` filename, audio-only flags (`-c:a aac -b:a 192k`)
      - Video: use `video/mp4` Content-Type, `segment.mp4` filename, use `-c:v copy -c:a copy` for fast keyframe-aligned cuts (near-instant, slightly imprecise at boundaries but acceptable for segment downloads)
    - Lines 396-403: Build ffmpeg command conditionally:
      - Audio: current flags (`-c:a aac -b:a 192k -f mp4 -movflags frag_keyframe+empty_moov`)
      - Video: `-c:v copy -c:a copy -f mp4 -movflags frag_keyframe+empty_moov`

  **Must NOT do**:
  - Do NOT re-encode video for segments (use stream copy for speed)
  - Do NOT change the segment API interface (same query params)
  - Do NOT change audio segment behavior

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with T13, T15, T16, T17, T18)
  - **Blocks**: T19
  - **Blocked By**: T6

  **References**:

  **Pattern References**:
  - `internal/recordings/handler.go:361-408` — Current `segmentRecording`. Lines 390 (playback.m4a), 393-394 (content-type, disposition), 396-403 (ffmpeg command).

  **Acceptance Criteria**:

  **QA Scenarios**:

  ```
  Scenario: Video segment download works
    Tool: Bash
    Steps:
      1. curl -o segment.mp4 "http://localhost:8080/api/recordings/{videoId}/segment?start=5&end=15" (with auth)
      2. Assert: HTTP 200
      3. Assert: Content-Type is video/mp4
      4. Run: ffprobe segment.mp4 — assert valid MP4 with video+audio streams
    Expected Result: Video segment downloaded as MP4
    Evidence: .sisyphus/evidence/task-14-video-segment.txt

  Scenario: Audio segment unchanged (regression)
    Tool: Bash
    Steps:
      1. curl -o segment.m4a "http://localhost:8080/api/recordings/{audioId}/segment?start=0&end=10"
      2. Assert: Content-Type is audio/mp4
      3. Assert: Content-Disposition contains segment.m4a
    Expected Result: Audio segments unchanged
    Evidence: .sisyphus/evidence/task-14-audio-segment.txt
  ```

  **Commit**: YES (groups with T13)
  - Message: `feat(recordings): add video stream, thumbnail, and segment handlers`
  - Files: `internal/recordings/handler.go`

- [ ] 15. Share Handler + ExtractSegment — Video Support

  **What to do**:
  - In `internal/shares/handler.go`:
    - `CreateShare` (lines 60-72): Replace hardcoded `"playback.m4a"` with `recordings.PlaybackFilename(rec.MediaType)` — need to look up the recording to get its media type. Also update `ExtractSegment` call to be media-type-aware.
    - `StreamShare` (lines 100-137):
      - For section shares (lines 115-126): Replace `"snippet.m4a"` with media-type-aware filename (`snippet.m4a` for audio, `snippet.mp4` for video). Branch content-type.
      - For full shares (lines 128-136): Replace `"playback.m4a"` with `PlaybackFilename`. Branch content-type.
    - `DownloadShare` (lines 165-204):
      - For section shares (lines 179-191): Replace `"snippet.m4a"` with media-type-aware filename. Branch content-type and disposition filename.
      - Full download (lines 193-203): Already uses `rec.MimeType` and `rec.FileExt` — no changes needed.
  - In `internal/recordings/processing.go`, update `ExtractSegment` (lines 195-223):
    - Accept a `mediaType` parameter
    - For audio: keep current behavior (`snippet.m4a`, audio-only ffmpeg flags, waveform generation)
    - For video: output `snippet.mp4`, use `-c:v copy -c:a copy` for fast segment copy, skip waveform generation
  - Add a helper: `SnippetFilename(mediaType string) string` → `"snippet.m4a"` or `"snippet.mp4"`

  **Must NOT do**:
  - Do NOT change share access control logic
  - Do NOT re-encode video for share segments (use stream copy)
  - Do NOT generate waveform for video share snippets

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with T13, T14, T16, T17, T18)
  - **Blocks**: T19
  - **Blocked By**: T6, T7

  **References**:

  **Pattern References**:
  - `internal/shares/handler.go:35-77` — `CreateShare`: Line 61 hardcodes `playback.m4a`. Line 68 calls `ExtractSegment`. Must look up recording's media_type.
  - `internal/shares/handler.go:100-137` — `StreamShare`: Lines 116, 123-124 hardcode `snippet.m4a` and `audio/mp4`. Lines 128, 135-136 hardcode `playback.m4a` and `audio/mp4`.
  - `internal/shares/handler.go:165-204` — `DownloadShare`: Lines 180, 187-189 hardcode `snippet.m4a` and `audio/mp4`.
  - `internal/recordings/processing.go:195-223` — `ExtractSegment`: Line 202 hardcodes `snippet.m4a`. Lines 205-211 use audio-only ffmpeg flags. Line 218 generates waveform.

  **Acceptance Criteria**:

  **QA Scenarios**:

  ```
  Scenario: Video share stream serves video/mp4
    Tool: Bash
    Steps:
      1. Create a share for a video recording (full share, no snippet)
      2. curl -I http://localhost:8080/api/s/{token}/stream
      3. Assert: Content-Type is video/mp4
    Expected Result: Video shares stream as video/mp4
    Evidence: .sisyphus/evidence/task-15-video-share-stream.txt

  Scenario: Video snippet share works
    Tool: Bash
    Steps:
      1. Create a share for a video recording with start_seconds=5, end_seconds=15
      2. curl -I http://localhost:8080/api/s/{token}/stream
      3. Assert: Content-Type is video/mp4
      4. Assert: response is a valid video file
    Expected Result: Video snippet extracted and served correctly
    Evidence: .sisyphus/evidence/task-15-video-snippet.txt

  Scenario: Audio share unchanged (regression)
    Tool: Bash
    Steps:
      1. Create a share for an audio recording
      2. curl -I http://localhost:8080/api/s/{token}/stream
      3. Assert: Content-Type is audio/mp4
    Expected Result: Audio shares unchanged
    Evidence: .sisyphus/evidence/task-15-audio-share.txt
  ```

  **Commit**: YES
  - Message: `feat(shares): update share handlers for video support`
  - Files: `internal/shares/handler.go`, `internal/recordings/processing.go`

- [ ] 16. RecordingDetail — Conditional Player Rendering

  **What to do**:
  - In `frontend/src/components/recordings/RecordingDetail.tsx`:
    - Import the new `VideoPlayer` component (from T10) and its ref type
    - At lines 96-102 (where `WaveformPlayer` renders): conditionally render:
      - `recording.media_type === 'video'` → `<VideoPlayer ref={videoRef} recording={recording} onTimeUpdate={setCurrentTime} />`
      - `recording.media_type === 'audio'` → `<WaveformPlayer ref={waveformRef} recording={recording} onTimeUpdate={setCurrentTime} />` (current behavior)
    - The `handleSeek` function (line 69-71) should call the correct ref's `seekTo` method
    - Update the segment download section: for video, use `segment.mp4` filename hint if desired (or keep as-is since the backend handles Content-Disposition)
    - Update metadata display: show video-specific metadata (resolution) when available, hide audio-specific fields (sample_rate, channels) when media_type is 'video'

  **Must NOT do**:
  - Do NOT change audio recording detail behavior
  - Do NOT remove the WaveformPlayer import (still used for audio)
  - Do NOT change the page layout structure

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with T13, T14, T15, T17, T18)
  - **Blocks**: None
  - **Blocked By**: T10

  **References**:

  **Pattern References**:
  - `frontend/src/components/recordings/RecordingDetail.tsx:96-102` — Where WaveformPlayer is rendered. This becomes the conditional rendering point.
  - `frontend/src/components/recordings/RecordingDetail.tsx:30-44` — State and refs. Add videoRef alongside waveformRef.
  - `frontend/src/components/recordings/RecordingDetail.tsx:69-71` — handleSeek function. Must call correct ref.
  - `frontend/src/components/recordings/RecordingDetail.tsx:104-125` — Metadata card. Add conditional video resolution display.

  **Acceptance Criteria**:

  **QA Scenarios**:

  ```
  Scenario: Video recording shows VideoPlayer
    Tool: Playwright
    Preconditions: A fully processed video recording exists
    Steps:
      1. Navigate to /recordings/{videoId}
      2. Assert: page contains a <video> element
      3. Assert: page does NOT contain a wavesurfer waveform container
      4. Assert: custom video controls are visible (play button, seek bar, volume)
      5. Screenshot
    Expected Result: Video recording renders with VideoPlayer, not WaveformPlayer
    Evidence: .sisyphus/evidence/task-16-video-detail.png

  Scenario: Audio recording still shows WaveformPlayer
    Tool: Playwright
    Steps:
      1. Navigate to /recordings/{audioId}
      2. Assert: page contains a waveform container (wavesurfer div)
      3. Assert: page does NOT contain a <video> element
    Expected Result: Audio recording unchanged — shows WaveformPlayer
    Evidence: .sisyphus/evidence/task-16-audio-detail.png

  Scenario: Song marker seek works with video player
    Tool: Playwright
    Steps:
      1. Navigate to video recording with song markers
      2. Click a song marker in the list
      3. Assert: video seeks to the marker's start_seconds
    Expected Result: Song marker click seeks video to correct time
    Evidence: .sisyphus/evidence/task-16-song-seek.png
  ```

  **Commit**: YES (groups with T12, T17, T18)
  - Message: `feat(frontend): video rendering in cards, detail, share, and song markers`
  - Files: `frontend/src/components/recordings/RecordingDetail.tsx`

- [ ] 17. SharePage — Video Support

  **What to do**:
  - In `frontend/src/pages/SharePage.tsx`:
    - Import `VideoPlayer` component
    - At lines 86-106 (where `WaveformPlayer` renders): conditionally render:
      - `recording.media_type === 'video'` → `<VideoPlayer recording={...} streamUrlOverride={shareStreamUrl(share.token)} />`
      - `recording.media_type === 'audio'` → `<WaveformPlayer ...>` (current behavior)
    - Update "Shared Audio" fallback text (line 59, 104) to be media-agnostic: "Shared Recording" or conditionally "Shared Video" / "Shared Audio"
    - The `shareStreamUrl` already resolves to the correct backend endpoint — the backend (T15) handles serving video/mp4 vs audio/mp4

  **Must NOT do**:
  - Do NOT change share access control or token logic
  - Do NOT add video-specific share features beyond player rendering
  - Do NOT change the page layout

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with T13, T14, T15, T16, T18)
  - **Blocks**: None
  - **Blocked By**: T10

  **References**:

  **Pattern References**:
  - `frontend/src/pages/SharePage.tsx:86-106` — Where WaveformPlayer renders with overrides. This becomes conditional.
  - `frontend/src/pages/SharePage.tsx:59` — "Shared Audio" text. Make media-agnostic.
  - `frontend/src/api/shares.ts:55-65` — Share URL helpers (shareStreamUrl, shareWaveformUrl). Already point to backend which will handle media type.

  **Acceptance Criteria**:

  **QA Scenarios**:

  ```
  Scenario: Video share renders with VideoPlayer
    Tool: Playwright
    Preconditions: A share exists for a video recording
    Steps:
      1. Navigate to /s/{shareToken}
      2. Assert: page contains a <video> element
      3. Assert: video is playable (click play, verify currentTime advances)
      4. Screenshot
    Expected Result: Video share page shows VideoPlayer
    Evidence: .sisyphus/evidence/task-17-video-share.png

  Scenario: Audio share unchanged
    Tool: Playwright
    Preconditions: A share exists for an audio recording
    Steps:
      1. Navigate to /s/{shareToken}
      2. Assert: page contains waveform (not video element)
    Expected Result: Audio shares unchanged
    Evidence: .sisyphus/evidence/task-17-audio-share.png

  Scenario: Video snippet share plays correct section
    Tool: Playwright
    Preconditions: A share exists for a video with start_seconds/end_seconds
    Steps:
      1. Navigate to /s/{snippetShareToken}
      2. Assert: video plays
      3. Assert: snippet badge shows correct time range
    Expected Result: Video snippet share renders correctly
    Evidence: .sisyphus/evidence/task-17-video-snippet.png
  ```

  **Commit**: YES (groups with T12, T16, T18)
  - Message: `feat(frontend): video rendering in cards, detail, share, and song markers`
  - Files: `frontend/src/pages/SharePage.tsx`

- [ ] 18. Song Markers — CSS Overlays on Video Scrubber

  **What to do**:
  - Song markers for video recordings need to be displayed as CSS-positioned overlays on the video player's seek bar (since there's no waveform)
  - In the `VideoPlayer` component (from T10) or `VideoControls`:
    - Accept an optional `songs` prop (array of Song objects with `start_seconds`)
    - Render small marker indicators (e.g., colored dots or thin vertical lines) on the seek/progress bar at the correct percentage positions: `(song.start_seconds / duration) * 100`%
    - Clicking a marker should seek to that song's `start_seconds`
    - Optionally show a tooltip with the song title on hover
  - In `RecordingDetail.tsx` (T16): pass the songs data to VideoPlayer if available
  - The `SongMarkerList` component (sidebar list) already works independently — it just needs the `onSeek` callback connected to the video player's seekTo, which T16 handles

  **Must NOT do**:
  - Do NOT use wavesurfer or any waveform library for video
  - Do NOT build a complex timeline editor — just simple position markers
  - Do NOT change the SongMarkerList component (it already handles CRUD, just needs seekTo callback)

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with T13, T14, T15, T16, T17)
  - **Blocks**: None
  - **Blocked By**: T10

  **References**:

  **Pattern References**:
  - `frontend/src/components/songs/SongMarkerList.tsx:1-85` — Existing song marker sidebar list. Uses `onSeek` callback. This component is NOT modified — it already works with any player that accepts seekTo.
  - `frontend/src/components/video/VideoControls.tsx` (from T10) — The seek bar where markers should be rendered as overlays.
  - `frontend/src/api/songs.ts` — Song type with `start_seconds` field (used for position calculation).

  **Acceptance Criteria**:

  **QA Scenarios**:

  ```
  Scenario: Song markers appear on video seek bar
    Tool: Playwright
    Preconditions: A video recording with 2+ song markers exists
    Steps:
      1. Navigate to /recordings/{videoId}
      2. Assert: the video seek bar shows marker indicators at positions matching song start_seconds
      3. Assert: markers are visually distinct (colored dots/lines)
      4. Screenshot
    Expected Result: Song markers visible on video scrubber at correct positions
    Evidence: .sisyphus/evidence/task-18-song-markers.png

  Scenario: Clicking a marker seeks video
    Tool: Playwright
    Steps:
      1. Click a song marker on the seek bar
      2. Assert: video currentTime jumps to the marker's start_seconds (±1s tolerance)
    Expected Result: Marker click seeks to correct time
    Evidence: .sisyphus/evidence/task-18-marker-click.png
  ```

  **Commit**: YES (groups with T12, T16, T17)
  - Message: `feat(frontend): video rendering in cards, detail, share, and song markers`
  - Files: `frontend/src/components/video/VideoPlayer.tsx` or `VideoControls.tsx`

- [ ] 19. Backend Go Tests — Video Handler Tests

  **What to do**:
  - In `internal/recordings/handler_test.go`:
    - Add test cases for video upload (ValidateMediaMagicBytes accepting MP4/MOV)
    - Add test cases for video streaming (correct Content-Type)
    - Add test cases for thumbnail endpoint (served when ready, 404 when not)
    - Add test cases for video segment (correct content-type and codec flags)
  - In `internal/shares/handler_test.go`:
    - Add test cases for video share streaming (correct content-type)
  - In `internal/recordings/validation_test.go` (create if doesn't exist):
    - Test ValidateMediaMagicBytes with all supported formats: MP3, FLAC, WAV, OGG, M4A, MP4, MOV
    - Test rejection of unsupported formats
    - Test ftyp brand parsing edge cases (different box sizes)
  - Follow existing test patterns in the codebase

  **Must NOT do**:
  - Do NOT modify existing passing tests
  - Do NOT add frontend tests (no test infrastructure)
  - Do NOT use `any` types in test helpers

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 4 (sequential, after all handlers)
  - **Blocks**: None
  - **Blocked By**: T9, T13, T14, T15

  **References**:

  **Pattern References**:
  - `internal/recordings/handler_test.go` — Existing handler tests. Follow same test setup, HTTP client patterns, assertions.
  - `internal/shares/handler_test.go` — Existing share handler tests. Follow same patterns.

  **Acceptance Criteria**:

  **QA Scenarios**:

  ```
  Scenario: All Go tests pass
    Tool: Bash
    Steps:
      1. Run: go test ./internal/recordings/... ./internal/shares/... -v
      2. Assert: all tests pass (exit code 0)
      3. Assert: new video-specific test cases are present and passing
    Expected Result: All tests green, no regressions
    Evidence: .sisyphus/evidence/task-19-go-tests.txt

  Scenario: Validation tests cover all formats
    Tool: Bash
    Steps:
      1. Run: go test ./internal/recordings/... -run TestValidateMedia -v
      2. Assert: tests for MP3, FLAC, WAV, OGG, M4A, MP4, MOV all pass
      3. Assert: unknown format rejection test passes
    Expected Result: Full format coverage in validation tests
    Evidence: .sisyphus/evidence/task-19-validation-tests.txt
  ```

  **Commit**: YES
  - Message: `test(recordings): add handler tests for video endpoints`
  - Files: `internal/recordings/handler_test.go`, `internal/shares/handler_test.go`, `internal/recordings/validation_test.go`

---

## Final Verification Wave

> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.

- [ ] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, curl endpoint, run command). For each "Must NOT Have": search codebase for forbidden patterns (hardcoded `playback.m4a`, `any` types, modified migration 001-003). Check evidence files exist in `.sisyphus/evidence/`. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [ ] F2. **Code Quality Review** — `unspecified-high`
  Run `go vet ./...`, `go test ./...`, and frontend `tsc -b && vite build`. Review all changed files for: `any` types, empty catches, `console.log` in prod, commented-out code, unused imports, hardcoded `playback.m4a`. Check AI slop: excessive comments, over-abstraction, generic variable names.
  Output: `Build [PASS/FAIL] | Lint [PASS/FAIL] | Tests [N pass/N fail] | Files [N clean/N issues] | VERDICT`

- [ ] F3. **Real Manual QA** — `unspecified-high` (+ `playwright` skill for UI)
  Start from clean state. Upload an actual MP4 video file, wait for processing to complete. Verify: thumbnail appears on card, video plays in detail page, song markers work, share link renders video player, segment download works. Upload an audio file and verify it still works identically. Save evidence to `.sisyphus/evidence/final-qa/`.
  Output: `Scenarios [N/N pass] | Integration [N/N] | Edge Cases [N tested] | VERDICT`

- [ ] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff. Verify 1:1 — everything in spec was built, nothing beyond spec was built. Check "Must NOT do" compliance. Detect cross-task contamination. Flag unaccounted changes.
  Output: `Tasks [N/N compliant] | Contamination [CLEAN/N issues] | Unaccounted [CLEAN/N files] | VERDICT`

---

## Commit Strategy

| After | Message | Files | Pre-commit |
|-------|---------|-------|------------|
| T1 | `feat(db): add video support migration 004` | `004_video_support.{up,down}.sql` | — |
| T5 | `feat(recordings): extend magic byte validation for video formats` | `validation.go` | `go vet ./internal/recordings/...` |
| T3+T2 | `feat(recordings): add video fields to structs and config` | `repository.go`, `processing.go`, `service.go`, `config.go` | `go vet ./...` |
| T6 | `feat(recordings): update repository queries for video columns` | `repository.go` | `go test ./internal/recordings/...` |
| T7+T8 | `feat(recordings): bifurcate processing pipeline and separate worker pools` | `processing.go`, `service.go` | `go vet ./...` |
| T9 | `feat(recordings): update upload handler for video support` | `handler.go` | `go test ./internal/recordings/...` |
| T13+T14 | `feat(recordings): add video stream, thumbnail, and segment handlers` | `handler.go` | `go test ./internal/recordings/...` |
| T15 | `feat(shares): update share handlers for video support` | `shares/handler.go`, `processing.go` | `go test ./internal/shares/...` |
| T4+T11 | `feat(frontend): update types and upload form for video` | `recordings.ts`, `shares.ts`, `UploadForm.tsx` | `tsc -b` |
| T10 | `feat(frontend): add VideoPlayer component with custom controls` | `VideoPlayer.tsx`, `useVideoPlayer.ts`, `VideoControls.tsx` | `tsc -b` |
| T12+T16+T17+T18 | `feat(frontend): video rendering in cards, detail, share, and song markers` | `RecordingCard.tsx`, `RecordingDetail.tsx`, `SharePage.tsx`, `SongMarkerOverlay.tsx` | `tsc -b && vite build` |
| T19 | `test(recordings): add handler tests for video endpoints` | `handler_test.go` | `go test ./internal/recordings/...` |

---

## Success Criteria

### Verification Commands
```bash
# Backend builds
go build ./...

# Backend tests pass
go test ./internal/recordings/... ./internal/shares/...

# Frontend compiles
cd frontend && npm run build

# Migration applies cleanly (dev)
migrate -path internal/database/migrations -database "$DATABASE_URL" up

# Video upload accepted (needs test MP4 file)
curl -X POST -F "file=@test.mp4" http://localhost:8080/api/recordings
# Expected: 201 with media_type: "video"

# Thumbnail served after processing
curl -I http://localhost:8080/api/recordings/{id}/thumbnail
# Expected: 200, Content-Type: image/jpeg

# Video stream served
curl -I http://localhost:8080/api/recordings/{id}/stream
# Expected: 200, Content-Type: video/mp4
```

### Final Checklist
- [ ] All "Must Have" present
- [ ] All "Must NOT Have" absent
- [ ] All Go tests pass
- [ ] Frontend builds without errors
- [ ] Audio recordings unchanged in behavior
- [ ] No hardcoded `playback.m4a` remaining (all use media-type-aware helper)
