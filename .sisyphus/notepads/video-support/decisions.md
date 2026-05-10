# Decisions — video-support

## 2026-05-10 — Atlas Setup

### Architecture Decisions

- **Media type field**: Use `string` type in Go struct (`"audio"` | `"video"`), DB CHECK constraint
- **PlaybackFilename helper**: Place in `processing.go` or new `media.go`, return `"playback.m4a"` for audio, `"playback.mp4"` for video
- **SnippetFilename helper**: Same pattern for snippets (`snippet.m4a` / `snippet.mp4`)
- **Worker pool**: Two separate semaphore channels in Service (`audioSem`, `videoSem`)
- **Video transcode**: H.264/AAC MP4 with faststart; copy if already H.264 at ≤1080p
- **Thumbnail**: Extract at 10% of duration (min 0.1s), scale 640px wide JPEG q80-85
- **No adaptive streaming, no waveform for video**
- **Upload size limit**: Use larger (video) limit for MaxBytesReader, validate actual type afterward
- **Song markers for video**: CSS position overlays on VideoControls seek bar
- **Video player**: Native `<video>` element with custom controls following WaveformPlayer pattern
- **No `any` types anywhere**
