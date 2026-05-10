# Learnings — video-support

## 2026-05-10 — Atlas Setup

### Codebase Conventions

**Go Backend:**
- Package: `recordings` for all recording-related code
- DB pool: `pgxpool.Pool` via `pgx/v5`
- Recording struct in `repository.go:14-36` with json tags + pointer-for-nullable pattern
- `CreateRecordingInput` struct at `repository.go:38-48`
- `ProcessingJob` struct at `processing.go:99-104`
- `ffprobeResult` at `processing.go:15-27` — currently only has CodecType, SampleRate, Channels in streams
- `processRecording` at `processing.go:109-175` — the main pipeline function
- `runFFmpeg` at `processing.go:50-65` — audio-only AAC transcode
- `runFFprobe` at `processing.go:29-48`
- `runAudiowaveform` at `processing.go:67-97` — skip for video
- Service struct at `service.go:12-19` — uses single semaphore chan
- `Enqueue(job ProcessingJob, timeoutSeconds int)` at `service.go:36-56`
- Config at `config.go` — uses `kelseyhightower/envconfig` with envconfig tags
- Validation at `validation.go:29` — function is `ValidateAudioMagicBytes`, returns `(mime, ext, reader, err)`

**Migrations:**
- 001_initial.up.sql — original recordings table (no media_type, no processing_step)
- 002_processing_step.up.sql — adds processing_step column with CHECK constraint
- Next migration number: 004

**Processing step values (current):** `'queued','analyzing','transcoding','generating_waveform','complete'`
**New step values (after T1):** add `'extracting_thumbnail'`

**Frontend:**
- React 19 + Radix UI + Tailwind CSS + Vite + wavesurfer.js 7.x + Uppy
- TypeScript types in `frontend/src/api/recordings.ts`
- WaveformPlayer: `frontend/src/components/audio/WaveformPlayer.tsx`
- AudioControls: `frontend/src/components/audio/AudioControls.tsx`
- ProcessingStatus: `frontend/src/components/audio/ProcessingStatus.tsx`
- RecordingCard: `frontend/src/components/recordings/RecordingCard.tsx`
- RecordingDetail: `frontend/src/components/recordings/RecordingDetail.tsx`
- SharePage: `frontend/src/pages/SharePage.tsx`
- Upload form: `frontend/src/components/recordings/UploadForm.tsx`
- Hooks: `frontend/src/hooks/useAudioPlayer.ts`

### Key Facts
- Current `ValidateAudioMagicBytes` uses prefix matching — ftyp M4A entries are hardcoded with specific box sizes
- Ftyp brand is at bytes 8-11 (after 4-byte size + 4-byte "ftyp")
- Existing `ftyp` entries in audioMagicBytes hardcode sizes 0x20 and 0x1C — these will be replaced by proper brand parsing
- `playback.m4a` is hardcoded in `processing.go:112` and share/stream handlers
- Single semaphore in service.go — needs to become two (audio + video)
- Upload limit: 500MB (config.UploadMaxSizeMB) — video needs 4GB

### Changes made (2026-05-10)
 - Added video-related TypeScript fields to Recording interface and CreateRecordingInput
 - Added 'extracting_thumbnail' processing step and updated ProcessingStatus component to include it
 - Added thumbnailUrl and shareThumbnailUrl helpers
 - Verified TypeScript build passes (npx tsc -b --noEmit)

- Added MediaType, ThumbnailReady, VideoWidth, VideoHeight to Recording struct in repository.go.
- Added MediaType to CreateRecordingInput in repository.go.
- Added MediaType to ProcessingJob and extended ffprobeResult streams with Width, Height, CodecName in processing.go.
- Added PlaybackFilename and SnippetFilename helpers in processing.go.

These changes compile; go build ./internal/recordings/... passed locally.

- T7: `processRecording` now branches on `job.MediaType`, preserving the audio pipeline while video uses MP4 playback generation plus thumbnail extraction without waveform generation.
