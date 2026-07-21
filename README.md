# Shakedown

A self-hosted recording library for managing, playing, and sharing audio and video recordings. Built with a Go backend and React frontend.

## Features

- Upload and stream audio/video recordings
- Waveform visualization for audio playback
- Video playback with seek bar and song markers
- Tag and organize recordings
- Timestamped comments
- Song/section markers with seek-to-position
- Shareable links with optional time-range snippets
- Segment download (extract a clip by start time + duration)
- OIDC authentication (Pocket ID) with optional bypass for local dev

## Architecture

```
├── cmd/server/          # Go entrypoint
├── internal/
│   ├── auth/            # OIDC + dev-mode auth
│   ├── config/          # Environment-based configuration
│   ├── database/        # PostgreSQL pool + embedded migrations
│   ├── recordings/      # Upload, storage, processing pipeline, streaming
│   ├── songs/           # Song/section markers
│   ├── comments/        # Timestamped comment threads
│   ├── tags/            # Tag CRUD
│   ├── shares/          # Share link generation + access
│   ├── admin/           # Admin endpoints
│   ├── middleware/       # HTTP middleware
│   └── static/          # Embedded frontend (production build)
├── frontend/            # React + TypeScript + Vite (shadcn/ui, Tailwind)
```

The Go server embeds the built frontend at compile time (`internal/static/dist`). In development, the Vite dev server proxies API requests to the Go backend.

### Processing pipeline

When a recording is uploaded, the server processes it through a background worker pipeline:

1. **Analyzing** — extract metadata (duration, channels, sample rate, bitrate, resolution)
2. **Transcoding** — normalize audio format via `ffmpeg`
3. **Generating waveform** — create waveform data via `audiowaveform`
4. **Extracting thumbnail** — for video files, extract a poster frame via `ffmpeg`

## Cloud sync (rclone)

Shakedown can automatically sync your recordings to any cloud storage provider supported by [rclone](https://rclone.org/).

### Prerequisites

- **Docker**: `rclone` is bundled in the official Docker image.
- **Non-Docker**: Install `rclone` as a system dependency (similar to `ffmpeg` and `audiowaveform`).

### Configuration

Every user recording is pushed to a single configured destination. This feature is instance-wide and restricted to administrators.

| Variable | Default | Description |
|---|---|---|
| `CLOUD_SYNC_ENABLED` | `false` | Enable/disable automatic cloud syncing |
| `CLOUD_SYNC_RCLONE_BIN` | `rclone` | Path to the rclone binary |
| `RCLONE_CONFIG` | `/data/rclone/rclone.conf` | Path to the rclone configuration file |
| `CLOUD_SYNC_REMOTE` | — | Name of the rclone remote to use |
| `CLOUD_SYNC_ROOT` | `Shakedown` | Root directory (or bucket name) at the remote |
| `CLOUD_SYNC_PATH_TEMPLATE` | `{year}/{date}/{title}.{ext}` | Path template for synced files |
| `CLOUD_SYNC_INTERVAL_SECONDS` | `3600` | How often to scan for unsynced recordings |
| `CLOUD_SYNC_MAX_WORKERS` | `2` | Concurrent sync workers |
| `CLOUD_SYNC_MAX_ATTEMPTS` | `5` | Max retry attempts per file |
| `CLOUD_SYNC_LEASE_TTL_SECONDS` | `900` | Internal lease duration for sync tasks |
| `CLOUD_SYNC_BACKOFF_BASE_SECONDS` | `60` | Base delay for exponential backoff |
| `CLOUD_SYNC_TPS_LIMIT` | `0` | rclone transactions per second limit (0 = unlimited) |

### Path templates

The `CLOUD_SYNC_PATH_TEMPLATE` supports these tokens:

- `{year}`: Year of recording (e.g. `2024`)
- `{month}`: Month of recording (e.g. `05`)
- `{date}`: Full date of recording (e.g. `2024-05-20`)
- `{title}`: Sanitized recording title
- `{ext}`: File extension (e.g. `mp3`, `mp4`)
- `{id}`: Recording's short ID

### Setup methods

1. **Admin UI**: Paste an rclone remote-config block (the output of `rclone config show <remote>`) into the Cloud Sync card on the `/admin` page.
2. **Manual**: Place a pre-configured `rclone.conf` at the path specified by `RCLONE_CONFIG`.

### Google Drive Example

1. Run `rclone config` locally.
2. Create a new remote of type `drive`.
3. Use the `drive.file` scope for least-privilege access (restricts Shakedown to files it creates).
4. **Important**: Using your own OAuth `client_id` (registered in Google Cloud Console) provides durable, non-expiring tokens. The rclone built-in client_id works out of the box but is rate-limited (~2 files/s) and intended for casual use.

### Security Notes

- **Secrets**: `rclone.conf` contains sensitive credentials. It should be stored securely (0600 permissions). NEVER commit this file or expose its contents in logs.
- **DISABLE_AUTH**: Do NOT enable cloud sync while `DISABLE_AUTH=true` is publicly reachable. In `internal/auth/middleware.go`, `DevAuth` makes every request a synthetic admin user:
  > DevAuth injects a synthetic local dev user into every request... Role: "admin"
  This means ANYONE could trigger syncs or reconfigure your remote if the instance is exposed.

### Destination Note

Point `CLOUD_SYNC_ROOT` at a **dedicated folder**.
- `rclone copyto` will overwrite a file if one already exists at the computed remote path.
- For bucket-style backends (S3, B2, GCS), the first segment of `CLOUD_SYNC_ROOT` is treated as the bucket name and **must already exist**.

Any rclone backend works (S3, Dropbox, OneDrive, SFTP, etc.). Google Drive is just one example.

## Prerequisites

| Dependency | Version | Notes |
|---|---|---|
| Go | 1.26+ | Backend |
| Node.js | 22+ | Frontend |
| PostgreSQL | 16+ | Database |
| ffmpeg | any recent | Audio/video processing |
| audiowaveform | 1.10+ | Waveform generation ([BBC/audiowaveform](https://github.com/bbc/audiowaveform)) |

## Quick start (Docker)

```sh
cp .env.example .env
docker compose up
```

This starts PostgreSQL, the Go backend (port 8080), and the Vite dev server (port 5173). Visit `http://localhost:5173`.

## Local development (without Docker)

### 1. Start PostgreSQL

Use Docker for just the database, or a local install:

```sh
docker compose up db
```

### 2. Configure environment

```sh
cp .env.example .env
```

For local dev, the defaults work as-is. `DISABLE_AUTH=true` bypasses OIDC and uses a synthetic dev user. Set `DATABASE_URL` to point at your local Postgres if not using the Docker default.

### 3. Start the backend

```sh
make dev
```

The server starts on port 8080. Database migrations run automatically on startup.

### 4. Start the frontend

```sh
cd frontend
npm install
npm run dev
```

The Vite dev server starts on port 5173 and proxies `/api` requests to `http://localhost:8080` (configurable via `VITE_API_PROXY`).

## Environment variables

See `.env.example` for all options. Key variables:

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | Server listen port |
| `DATABASE_URL` | (see .env.example) | PostgreSQL connection string |
| `STORAGE_ROOT` | `/data/audio` | Directory for uploaded files |
| `DISABLE_AUTH` | `true` | Bypass OIDC for local development |
| `OIDC_ISSUER` | — | OIDC provider URL (e.g. Pocket ID instance) |
| `OIDC_CLIENT_ID` | — | OIDC client ID |
| `OIDC_CLIENT_SECRET` | — | OIDC client secret |
| `SESSION_SECRET` | — | 32-byte secret for session cookies |
| `PROCESSING_MAX_WORKERS` | `4` | Concurrent audio processing workers |
| `UPLOAD_MAX_SIZE_MB` | `500` | Max upload file size |

## Production build

```sh
cd frontend && npm ci && npm run build && cd ..
go build -o shakedown ./cmd/server/
```

The frontend is built into `frontend/dist/`, then embedded into the Go binary via `internal/static/`. The single binary serves both the API and the SPA.

Alternatively, use the multi-stage Dockerfile:

```sh
docker build -t shakedown .
```

## Commands

| Command | Description |
|---|---|
| `make dev` | Run the Go backend in dev mode |
| `make build` | Compile the Go backend |
| `make test` | Run Go tests |
| `make lint` | Run linter (golangci-lint or go vet) |
| `npm run dev` | Start Vite dev server (from `frontend/`) |
| `npm run build` | Build frontend for production (from `frontend/`) |
| `npm run lint` | Lint frontend (from `frontend/`) |
