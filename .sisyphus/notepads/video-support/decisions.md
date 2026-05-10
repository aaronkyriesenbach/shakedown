# Decisions — video-support

## 2026-05-10

- Added media_type string to Recording and CreateRecordingInput. Non-nullable; will be set to "audio" for current uploads until video path enabled.
- Added ThumbnailReady bool and nullable VideoWidth/VideoHeight ints to Recording to track thumbnail generation and video dimensions.
- Added MediaType to ProcessingJob so workers know whether to transcode audio or handle video paths.
- Added PlaybackFilename and SnippetFilename helpers to centralize file naming for audio/video.
