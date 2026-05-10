# Decisions

## Architecture
- Single multi-stage Docker image: node build -> go build -> debian-slim runtime
- Go binary embeds the frontend build via embed.FS
- All config via environment variables (12-factor)
- DB-backed sessions for multi-replica readiness

## API Design
- All auth-required endpoints under /api/* protected by session cookie middleware
- Public share endpoints under /api/s/:token with separate token-only middleware
- Admin endpoints at /api/admin/* require admin role

## Processing
- Goroutine pool with semaphore (configurable max workers, default 4)
- Job sequence: ffprobe -> ffmpeg transcode -> audiowaveform
- recorded_at fallback: embedded tags -> filename date -> upload timestamp
- Timeout: context cancellation for hung subprocesses (default 5 min)

## Security
- SafeJoin helper for all file operations (path traversal prevention)
- Share tokens: 32 crypto-random bytes, base64url
- Sessions: httpOnly, Secure, SameSite=Strict cookies
- CSRF: SameSite=Strict + Origin header validation
