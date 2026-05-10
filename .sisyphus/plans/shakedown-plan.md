# Shakedown Implementation Plan

> **Provenance**: This plan was produced by an adversarial multi-agent planning session (5 members, 3 rounds of cross-critique) followed by a plan agent synthesis pass. All architectural decisions survived at least 3 rounds of hostile review.

---

## Tech Stack (Settled)

| Layer | Technology | Justification |
|-------|-----------|---------------|
| Backend | Go 1.22+ / `chi` router | Single binary, excellent concurrent Range-request streaming, goroutines for background processing, 20MB Docker image |
| Frontend | React 18 + Vite + TypeScript (strict) | Widest ecosystem for audio UI (wavesurfer.js React adapter), shadcn/ui + Tailwind CSS, TanStack Query |
| Database | PostgreSQL 16 / `pgx` v5 | Concurrent writes, recursive CTEs for threaded comments, full-text search, UUID support. `golang-migrate` for migrations |
| Audio Processing | ffmpeg + ffprobe (subprocesses) | Metadata extraction, AAC derivative transcoding, segment extraction |
| Waveform Generation | BBC `audiowaveform` binary | Server-side peaks JSON generation (~40-80KB per recording) |
| Waveform Rendering | wavesurfer.js v7 | Precomputed peaks loading, plugin ecosystem (regions, timeline) |
| Upload UX | Uppy (frontend only) | Drag-and-drop, progress, file validation — against standard multipart endpoint (no TUS) |
| Auth | Pocket ID (OIDC) / `coreos/go-oidc/v3` | Standard Authorization Code + PKCE flow, server-side token exchange |
| Design System | shadcn/ui + Tailwind CSS | Dark/light mode via CSS custom properties, accessible, no vendor lock-in |
| Config | Environment variables (12-factor) | `envconfig` or similar Go library, optional `.env` file for local dev |
| Deployment | Single multi-stage Docker image | Go binary + `embed.FS` frontend, debian-slim + ffmpeg + audiowaveform, UID/GID 1000 |

---

## Project Structure

```
shakedown/
  cmd/
    server/
      main.go                    # Entry point: config load, DB connect, router setup, server start
  internal/
    config/
      config.go                  # Struct + env var parsing (OIDC, DB, storage, server settings)
    database/
      database.go                # pgxpool setup, health check
      migrations/
        001_initial.up.sql       # Full schema
        001_initial.down.sql
    auth/
      middleware.go              # OIDC session validation middleware, permission stub
      handler.go                 # /login, /callback, /logout, /me endpoints
      session.go                 # DB-backed session CRUD
      oidc.go                    # OIDC provider setup, token exchange
    recordings/
      handler.go                 # CRUD, upload (multipart streaming), stream, download, segment
      service.go                 # Business logic, validation
      repository.go              # SQL queries (pgx)
      storage.go                 # Storage interface + LocalStorage implementation
      processing.go              # Async pipeline: ffprobe, transcode, waveform
      validation.go              # Magic-byte validation, filename sanitization
    songs/
      handler.go                 # CRUD for song markers/timestamps
      repository.go
    comments/
      handler.go                 # CRUD, threaded comments
      repository.go
    tags/
      handler.go                 # CRUD tags, recording-tag associations
      repository.go
    shares/
      handler.go                 # Create shares (auth), public access (token-only)
      middleware.go              # Token validation middleware (no OIDC dependency)
      repository.go
    admin/
      handler.go                 # Data dump (ZIP stream), user management
    middleware/
      cors.go                    # CORS (if needed)
      logging.go                 # Request logging
      ratelimit.go               # Rate limiting for public endpoints
    static/
      dist/                      # Embedded frontend build (via embed.FS)
  frontend/
    package.json
    vite.config.ts
    tsconfig.json
    tailwind.config.ts
    src/
      main.tsx                   # React entry, router setup
      App.tsx                    # Root layout, theme provider, auth context
      api/
        client.ts                # Fetch wrapper, auth interceptor
        recordings.ts            # Recording API hooks (TanStack Query)
        songs.ts                 # Song marker API hooks
        comments.ts              # Comment API hooks
        tags.ts                  # Tag API hooks
        shares.ts                # Share API hooks
        auth.ts                  # Auth API hooks (/me, login redirect)
        admin.ts                 # Admin API hooks
      components/
        ui/                      # shadcn/ui components (button, dialog, input, etc.)
        layout/
          AppLayout.tsx          # Sidebar + content area
          Header.tsx             # Top bar, user menu, theme toggle
          MobileNav.tsx          # Bottom nav for mobile
        audio/
          WaveformPlayer.tsx     # wavesurfer.js wrapper with precomputed peaks
          AudioControls.tsx      # Play/pause, seek, volume, time display
          ProcessingStatus.tsx   # "Processing..." state for uploads in progress
        recordings/
          RecordingList.tsx      # Date-grouped recording list
          RecordingCard.tsx      # Single recording summary card
          RecordingDetail.tsx    # Full recording view with player + comments + songs
          UploadForm.tsx         # Uppy-powered multi-file upload
          RecordingEditDialog.tsx # Edit title, date, tags
        songs/
          SongMarkerList.tsx     # Song timestamps sidebar
          SongMarkerForm.tsx     # Add/edit song marker
        comments/
          CommentThread.tsx      # Threaded comment display
          CommentForm.tsx        # New comment input
          CommentAtTimestamp.tsx  # Comment linked to playback position
        tags/
          TagFilter.tsx          # Tag filter bar
          TagManager.tsx         # Create/edit tags
        shares/
          ShareDialog.tsx        # Create share link dialog
          SnippetPlayer.tsx      # Minimal public share player
        admin/
          AdminDump.tsx          # Trigger and download data dump
          UserManagement.tsx     # View users, set admin role
        auth/
          LoginPage.tsx          # Login redirect
          AuthGuard.tsx          # Protected route wrapper
      hooks/
        useTheme.ts              # Dark/light mode state
        useAudioPlayer.ts        # Global audio player state
        useAuth.ts               # Auth state from /me endpoint
      pages/
        LibraryPage.tsx          # Browse recordings (default view)
        RecordingPage.tsx        # Single recording detail
        UploadPage.tsx           # Upload flow
        SharePage.tsx            # Public share view (unauthenticated)
        AdminPage.tsx            # Admin panel
        LoginPage.tsx            # Login redirect
      lib/
        utils.ts                 # Formatters, helpers
        theme.ts                 # Theme constants
  Dockerfile
  docker-compose.yml             # Local dev: postgres + app
  .env.example
  go.mod
  go.sum
  Makefile                       # Build, dev, test, lint commands
  README.md
```

---

## Data Model

```sql
-- USERS
CREATE TABLE users (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  oidc_sub     TEXT NOT NULL UNIQUE,
  email        TEXT NOT NULL,
  display_name TEXT NOT NULL,
  avatar_url   TEXT,
  role         TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('user','admin')),
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- RECORDINGS
CREATE TABLE recordings (
  id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  title               TEXT NOT NULL,
  original_filename   TEXT NOT NULL,
  storage_path        TEXT NOT NULL UNIQUE,
  file_size           BIGINT NOT NULL,
  duration_seconds    DOUBLE PRECISION,
  mime_type           TEXT NOT NULL,
  bitrate             INTEGER,
  sample_rate         INTEGER,
  channels            SMALLINT,
  recorded_at         TIMESTAMPTZ NOT NULL,
  recorded_at_source  TEXT NOT NULL DEFAULT 'upload_time'
                      CHECK (recorded_at_source IN ('metadata','user_set','upload_time','filename')),
  playback_ready      BOOLEAN NOT NULL DEFAULT false,
  waveform_ready      BOOLEAN NOT NULL DEFAULT false,
  uploaded_by         UUID NOT NULL REFERENCES users(id),
  created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at          TIMESTAMPTZ
);
CREATE INDEX idx_recordings_recorded_at ON recordings(recorded_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX idx_recordings_uploaded_by ON recordings(uploaded_by);

-- TAGS
CREATE TABLE tags (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name       TEXT NOT NULL UNIQUE,
  color      TEXT,
  created_by UUID NOT NULL REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE recording_tags (
  recording_id UUID NOT NULL REFERENCES recordings(id) ON DELETE CASCADE,
  tag_id       UUID NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
  tagged_by    UUID NOT NULL REFERENCES users(id),
  tagged_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (recording_id, tag_id)
);

-- SONGS (timestamp markers within a recording)
CREATE TABLE songs (
  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  recording_id     UUID NOT NULL REFERENCES recordings(id) ON DELETE CASCADE,
  title            TEXT NOT NULL,
  start_seconds    DOUBLE PRECISION NOT NULL,
  end_seconds      DOUBLE PRECISION,
  notes            TEXT,
  created_by       UUID NOT NULL REFERENCES users(id),
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_songs_recording ON songs(recording_id, start_seconds);

-- COMMENTS (threaded)
CREATE TABLE comments (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  recording_id      UUID NOT NULL REFERENCES recordings(id) ON DELETE CASCADE,
  song_id           UUID REFERENCES songs(id) ON DELETE SET NULL,
  parent_id         UUID REFERENCES comments(id) ON DELETE CASCADE,
  timestamp_seconds DOUBLE PRECISION,
  content           TEXT NOT NULL,
  author_id         UUID NOT NULL REFERENCES users(id),
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at        TIMESTAMPTZ
);
CREATE INDEX idx_comments_recording ON comments(recording_id, timestamp_seconds);
CREATE INDEX idx_comments_parent ON comments(parent_id);

-- SHARES
CREATE TABLE shares (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  token           TEXT NOT NULL UNIQUE,
  recording_id    UUID NOT NULL REFERENCES recordings(id) ON DELETE CASCADE,
  song_id         UUID REFERENCES songs(id) ON DELETE SET NULL,
  start_seconds   DOUBLE PRECISION,
  end_seconds     DOUBLE PRECISION,
  label           TEXT,
  created_by      UUID NOT NULL REFERENCES users(id),
  expires_at      TIMESTAMPTZ,
  access_count    INTEGER NOT NULL DEFAULT 0,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- SESSIONS (DB-backed for multi-replica readiness)
CREATE TABLE sessions (
  id         TEXT PRIMARY KEY,
  user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  data       JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX idx_sessions_user ON sessions(user_id);
CREATE INDEX idx_sessions_expires ON sessions(expires_at);
```

### Storage Layout

```
/data/audio/
  {recording_id}/
    original.{ext}         # Immutable original upload
    playback.m4a           # AAC 192kbps derivative (universal browser playback)
    waveform.json          # BBC audiowaveform peaks data
```

---

## API Routes

```
# AUTH
GET    /api/auth/login                    # Redirect to Pocket ID
GET    /api/auth/callback                 # OIDC callback, create session
POST   /api/auth/logout                   # Delete session
GET    /api/auth/me                       # Current user info

# RECORDINGS (auth required)
GET    /api/recordings                    # List (filters: ?tag, ?q, ?from, ?to, ?sort)
POST   /api/recordings                   # Upload (multipart, streaming)
GET    /api/recordings/:id                # Detail (includes processing status)
PATCH  /api/recordings/:id                # Update (title, recorded_at)
DELETE /api/recordings/:id                # Soft delete

GET    /api/recordings/:id/stream         # Range-request playback (serves playback.m4a)
GET    /api/recordings/:id/download       # Original file download
GET    /api/recordings/:id/waveform       # Peaks JSON
GET    /api/recordings/:id/segment        # ?start=&end= -> ffmpeg pipe (AAC output)

# SONGS (auth required)
GET    /api/recordings/:id/songs          # List song markers
POST   /api/recordings/:id/songs          # Create marker
PATCH  /api/recordings/:id/songs/:sid     # Update marker
DELETE /api/recordings/:id/songs/:sid     # Delete marker

# COMMENTS (auth required)
GET    /api/recordings/:id/comments       # List (threaded)
POST   /api/recordings/:id/comments       # Create (with optional parent_id, timestamp_seconds)
PATCH  /api/recordings/:id/comments/:cid  # Update content
DELETE /api/recordings/:id/comments/:cid  # Soft delete

# TAGS (auth required)
GET    /api/tags                          # List all tags
POST   /api/tags                          # Create tag
POST   /api/recordings/:id/tags           # Attach tag
DELETE /api/recordings/:id/tags/:tid      # Detach tag

# SHARES (auth required to create; token-only to access)
POST   /api/shares                        # Create share link
GET    /api/s/:token                      # Public: share metadata + recording info
GET    /api/s/:token/stream               # Public: Range-request stream
GET    /api/s/:token/download             # Public: download segment/recording
GET    /api/s/:token/comments             # Public: read-only comments

# ADMIN (admin role required)
GET    /api/admin/dump                    # Stream ZIP: all audio + JSON metadata
GET    /api/admin/users                   # List users
PATCH  /api/admin/users/:id              # Update role
```

---

## Implementation Phases

### Wave 1: Project Scaffolding (3 tasks, parallelizable)

**Gate**: `go build` succeeds, `npm run build` succeeds, `docker build` succeeds (empty app)

#### Task 1.1: Go Backend Scaffold
- Initialize Go module (`go mod init`)
- Create `cmd/server/main.go` with basic HTTP server using chi
- Add health check endpoint (`GET /api/health`)
- Set up `internal/` package structure (empty packages with doc comments)
- Add `Makefile` with targets: `build`, `dev`, `test`, `lint`
- **Files**: `go.mod`, `cmd/server/main.go`, `Makefile`, `internal/` package stubs

#### Task 1.2: React Frontend Scaffold
- Initialize Vite + React + TypeScript project in `frontend/`
- Configure `tsconfig.json` (strict mode, path aliases `@/`)
- Install and configure Tailwind CSS
- Install shadcn/ui, initialize with dark theme
- Create minimal `App.tsx` with router placeholder
- **Files**: `frontend/` entire scaffold, `package.json`, `vite.config.ts`, `tsconfig.json`, `tailwind.config.ts`

#### Task 1.3: Docker + Dev Environment
- Create multi-stage `Dockerfile` (node build -> go build -> debian-slim runtime with ffmpeg)
- Create `docker-compose.yml` (postgres:16-alpine + app)
- Create `.env.example` with all config vars documented
- Add `.dockerignore`
- **Files**: `Dockerfile`, `docker-compose.yml`, `.env.example`, `.dockerignore`

---

### Wave 2: Foundation (5 tasks, parallelizable)

**Gate**: DB migrations run, config loads from env, storage writes/reads files, frontend shows a themed shell

#### Task 2.1: Config Module
- Define config struct with all settings (DB URL, OIDC issuer/client, storage root, server port, upload max size)
- Parse from environment variables
- Validate required fields at startup
- **Files**: `internal/config/config.go`

#### Task 2.2: Database Module
- Set up `pgxpool` connection with config
- Create migration files with full schema (all tables from data model)
- Integrate `golang-migrate` with `embed.FS` source driver
- Auto-run migrations on startup
- Add DB health check
- **Files**: `internal/database/database.go`, `internal/database/migrations/*.sql`

#### Task 2.3: HTTP Server Setup
- Configure chi router with middleware stack (logging, recovery, request ID)
- Mount domain subrouters at `/api/auth`, `/api/recordings`, etc.
- Serve embedded frontend static files for non-API routes (SPA fallback to index.html)
- Add graceful shutdown
- **Files**: `cmd/server/main.go` (expand), `internal/middleware/logging.go`

#### Task 2.4: Storage Module
- Define `Storage` interface: `Write(ctx, path, io.Reader)`, `Read(ctx, path) (io.ReadSeekCloser, int64, error)`, `Delete(ctx, path)`, `Exists(ctx, path)`
- Implement `LocalStorage` using `os` package
- Include `SafeJoin` helper (path traversal prevention)
- Create storage root directory on startup if missing
- **Files**: `internal/recordings/storage.go`

#### Task 2.5: Frontend Layout Shell
- Create `AppLayout` with sidebar navigation
- Create `Header` with placeholder user menu and theme toggle
- Create dark/light mode toggle using CSS custom properties + `prefers-color-scheme`
- Set up React Router with placeholder pages
- Configure TanStack Query provider
- Create API client wrapper (`fetch` with credentials)
- **Files**: `frontend/src/App.tsx`, `frontend/src/components/layout/*`, `frontend/src/hooks/useTheme.ts`, `frontend/src/api/client.ts`, `frontend/src/pages/*` (stubs)

---

### Wave 3: Auth + Upload Core (3 tasks, partially parallelizable)

**Dependency**: Wave 2 (config, DB, HTTP, storage)
**Gate**: User can log in via Pocket ID, session persists across refreshes, files upload to disk with correct path structure

#### Task 3.1: OIDC Auth Backend
- Set up go-oidc provider discovery from config
- Implement Authorization Code + PKCE flow:
  - `GET /api/auth/login` — generate state/nonce, redirect to Pocket ID
  - `GET /api/auth/callback` — validate state, exchange code for tokens, upsert user, create session
  - `POST /api/auth/logout` — delete session, clear cookie
  - `GET /api/auth/me` — return current user from session
- DB-backed session management (httpOnly, Secure, SameSite=Strict cookies)
- Auth middleware: validate session cookie, inject user into request context
- Permission middleware stub: always returns true for authenticated users, checks admin role for admin endpoints
- **Files**: `internal/auth/oidc.go`, `internal/auth/handler.go`, `internal/auth/session.go`, `internal/auth/middleware.go`

#### Task 3.2: Recording Upload Endpoint
- `POST /api/recordings` — multipart streaming upload
- Stream file to temp path (never buffer full file in memory)
- Validate magic bytes (first 32 bytes: MP3, FLAC, WAV, OGG, M4A)
- Atomic rename from temp to `{recording_id}/original.{ext}`
- Extract basic info: file size, MIME type, original filename (sanitized)
- Insert recording row with `playback_ready=false`, `waveform_ready=false`
- Return 202 Accepted with recording ID
- Support multiple files in single request
- **Files**: `internal/recordings/handler.go`, `internal/recordings/validation.go`, `internal/recordings/repository.go`

#### Task 3.3: Frontend Theme System
- Implement dark/light mode with system preference detection
- Create theme toggle component in header
- Configure shadcn/ui components for both themes
- Design token system: colors, spacing, typography (music/studio aesthetic — dark charcoal primary, clean accents)
- **Files**: `frontend/src/hooks/useTheme.ts`, `frontend/src/lib/theme.ts`, `frontend/src/components/ui/*` (shadcn setup)

---

### Wave 4: Processing Pipeline + Domain APIs (6 tasks, parallelizable)

**Dependency**: Wave 3 (auth, upload, theme)
**Gate**: Uploaded audio produces playback.m4a + waveform.json async. Songs, comments, tags CRUD works via API.

#### Task 4.1: Auth UI
- Login page with redirect to OIDC
- Auth guard (protected route wrapper)
- User menu in header (display name, avatar, logout)
- Loading state while checking `/api/auth/me`
- **Files**: `frontend/src/pages/LoginPage.tsx`, `frontend/src/components/auth/AuthGuard.tsx`, `frontend/src/hooks/useAuth.ts`, `frontend/src/api/auth.ts`

#### Task 4.2: Async Processing Pipeline
- Bounded goroutine pool (semaphore, configurable max workers, default 4)
- Job sequence on upload completion:
  1. **ffprobe**: Extract duration, bitrate, sample rate, channels, embedded date tags -> update recording row
  2. **ffmpeg transcode**: `ffmpeg -i original.{ext} -c:a aac -b:a 192k -movflags +faststart playback.m4a` -> set `playback_ready=true`
  3. **audiowaveform**: `audiowaveform --input-filename original.{ext} --output-format json --pixels-per-second 10 -o waveform.json` -> set `waveform_ready=true`
- Timeout handling (context cancellation for hung subprocesses)
- Error logging (failed processing should not crash server, recording stays in processing state)
- `recorded_at` fallback order: embedded date tags -> filename date parsing -> upload timestamp
- **Files**: `internal/recordings/processing.go`, `internal/recordings/service.go`

#### Task 4.3: Songs/Timestamps API
- CRUD for song markers on a recording
- Fields: title, start_seconds (required), end_seconds (nullable), notes
- Validate: start_seconds < end_seconds (if end provided), start_seconds < recording duration
- Return ordered by start_seconds
- **Files**: `internal/songs/handler.go`, `internal/songs/repository.go`

#### Task 4.4: Comments API
- CRUD for threaded comments on a recording
- Fields: content, timestamp_seconds (nullable), song_id (nullable), parent_id (nullable)
- Soft delete (set deleted_at)
- Return threaded structure (recursive CTE or app-level tree building)
- **Files**: `internal/comments/handler.go`, `internal/comments/repository.go`

#### Task 4.5: Tags API
- CRUD for tags (name, color)
- Attach/detach tags from recordings
- List recordings filtered by tag
- **Files**: `internal/tags/handler.go`, `internal/tags/repository.go`

#### Task 4.6: Recording List + Detail API
- `GET /api/recordings` — paginated, filterable (tag, date range, search query), sorted by recorded_at
- `GET /api/recordings/:id` — full detail including processing status, songs count, comments count
- `PATCH /api/recordings/:id` — update title, recorded_at (set recorded_at_source='user_set')
- `DELETE /api/recordings/:id` — soft delete
- Full-text search on title using PostgreSQL `tsvector`
- **Files**: `internal/recordings/handler.go` (expand), `internal/recordings/repository.go` (expand)

---

### Wave 5: Playback + UI Features + Shares (7 tasks, parallelizable)

**Dependency**: Wave 4 (processing pipeline, domain APIs)
**Gate**: Audio plays with waveform, upload UI works, library browses/filters, shares create and resolve

#### Task 5.1: Audio Streaming Endpoint
- `GET /api/recordings/:id/stream` — serve `playback.m4a` with Range request support
- Use Go's `http.ServeContent` for automatic Range header handling (206 Partial Content)
- `GET /api/recordings/:id/download` — serve `original.{ext}` with Content-Disposition: attachment
- `GET /api/recordings/:id/waveform` — serve `waveform.json`
- `GET /api/recordings/:id/segment` — pipe ffmpeg output: `ffmpeg -ss {start} -i playback.m4a -t {duration} -c:a aac -b:a 192k -f mp4 pipe:1`
- **Files**: `internal/recordings/handler.go` (expand)

#### Task 5.2: Waveform Peaks Endpoint
- Serve precomputed `waveform.json` from storage
- Handle case where waveform is not yet ready (return 202 with retry-after header)
- **Files**: `internal/recordings/handler.go` (expand)

#### Task 5.3: Upload UI (Uppy)
- Install Uppy with Dashboard + XHR Upload plugins
- Configure allowed file types (audio/*)
- Progress bars per file
- Optional recorded_at date picker
- Redirect to recording detail on completion
- Show processing status after upload
- **Files**: `frontend/src/components/recordings/UploadForm.tsx`, `frontend/src/pages/UploadPage.tsx`, `frontend/src/api/recordings.ts`

#### Task 5.4: Library/Browse UI
- Recording list grouped by date (month/day sections)
- Tag filter bar (multi-select)
- Search input (debounced, full-text)
- Date range filter
- Recording cards showing: title, duration, date, tags, processing status
- Empty states
- **Files**: `frontend/src/pages/LibraryPage.tsx`, `frontend/src/components/recordings/RecordingList.tsx`, `frontend/src/components/recordings/RecordingCard.tsx`, `frontend/src/components/tags/TagFilter.tsx`

#### Task 5.5: Tags UI
- Tag creation dialog (name + color picker)
- Tag pills on recording cards
- Tag management in recording edit dialog
- **Files**: `frontend/src/components/tags/TagManager.tsx`, `frontend/src/components/recordings/RecordingEditDialog.tsx`

#### Task 5.6: Shares Backend
- `POST /api/shares` — create share token (32 crypto-random bytes, base64url)
- Separate chi router for `/api/s/` with token-only middleware (no OIDC dependency)
- `GET /api/s/:token` — return share metadata + recording info
- `GET /api/s/:token/stream` — Range-request stream of relevant segment/recording
- `GET /api/s/:token/download` — download segment/recording
- `GET /api/s/:token/comments` — read-only comments
- Rate limiting: 100 req/min per IP on public endpoints
- Increment access_count on each access
- Respect expires_at
- **Files**: `internal/shares/handler.go`, `internal/shares/middleware.go`, `internal/shares/repository.go`, `internal/middleware/ratelimit.go`

#### Task 5.7: Admin Backend
- `GET /api/admin/dump` — stream ZIP containing:
  - All original audio files
  - `metadata.json` with all recordings, songs, comments, tags, shares
  - Organized by recording with human-readable structure
- `GET /api/admin/users` — list all users
- `PATCH /api/admin/users/:id` — update role (user/admin)
- Admin role check via permission middleware
- **Files**: `internal/admin/handler.go`

---

### Wave 6: Core UI Components (3 tasks, parallelizable)

**Dependency**: Wave 5 (streaming, waveform, shares, admin endpoints)
**Gate**: Full recording detail page with waveform player works. Library browse is functional. Admin panel works.

#### Task 6.1: Waveform Player Component
- wavesurfer.js v7 integration with React
- Load precomputed peaks from `/api/recordings/:id/waveform`
- Play/pause, seek (click on waveform), volume control
- Time display (current / total)
- Keyboard shortcuts: space (play/pause), left/right arrows (skip 5s)
- Handle `playback_ready=false` state (show processing spinner)
- Handle `waveform_ready=false` state (show basic audio player without waveform, upgrade when ready)
- **Files**: `frontend/src/components/audio/WaveformPlayer.tsx`, `frontend/src/components/audio/AudioControls.tsx`, `frontend/src/components/audio/ProcessingStatus.tsx`, `frontend/src/hooks/useAudioPlayer.ts`

#### Task 6.2: Recording Detail + Browse Pages
- Full recording detail page: waveform player + songs sidebar + comments section + tags + metadata
- Edit dialog for title and recorded_at
- Download button (original file)
- Segment download with start/end time inputs
- Share creation dialog
- **Files**: `frontend/src/pages/RecordingPage.tsx`, `frontend/src/components/recordings/RecordingDetail.tsx`

#### Task 6.3: Admin UI
- Admin panel page (behind admin role check)
- Data dump trigger + download
- User list with role toggle
- **Files**: `frontend/src/pages/AdminPage.tsx`, `frontend/src/components/admin/AdminDump.tsx`, `frontend/src/components/admin/UserManagement.tsx`

---

### Wave 7: Collaboration UI (3 tasks, parallelizable)

**Dependency**: Wave 6 (waveform player, recording detail page)
**Gate**: Users can create song markers, leave timestamped threaded comments, and share recordings with public links

#### Task 7.1: Song Markers UI
- Song marker list in recording detail sidebar
- Click marker to seek player to that timestamp
- Add/edit/delete song markers (start, optional end, title, notes)
- Visual markers on waveform timeline (wavesurfer.js regions plugin)
- **Files**: `frontend/src/components/songs/SongMarkerList.tsx`, `frontend/src/components/songs/SongMarkerForm.tsx`, `frontend/src/api/songs.ts`

#### Task 7.2: Comments UI
- Threaded comment display below/alongside waveform
- "Comment at current time" — captures current playback position
- Click comment timestamp to seek player
- Reply to comment (one level of threading)
- Edit/delete own comments
- **Files**: `frontend/src/components/comments/CommentThread.tsx`, `frontend/src/components/comments/CommentForm.tsx`, `frontend/src/components/comments/CommentAtTimestamp.tsx`, `frontend/src/api/comments.ts`

#### Task 7.3: Share UI + Snippet Player
- Share dialog: select recording or segment (using song markers or manual start/end)
- Copy share link button
- Public share page (`/s/:token`): minimal Snippet Player
  - Focused waveform player for the shared segment/recording
  - Read-only comments display
  - No navigation chrome — just the player and metadata
  - Works without authentication
- **Files**: `frontend/src/components/shares/ShareDialog.tsx`, `frontend/src/components/shares/SnippetPlayer.tsx`, `frontend/src/pages/SharePage.tsx`, `frontend/src/api/shares.ts`

---

### Wave 8: Polish + Production Readiness (4 tasks, parallelizable)

**Dependency**: Wave 7 (all UI features complete)
**Gate**: Mobile fully usable, Docker image production-ready, security hardened, happy-path E2E works

#### Task 8.1: Mobile Optimization
- Responsive layout (sidebar collapses to bottom nav)
- Touch-friendly waveform interactions
- Bottom-sheet comments on mobile
- Upload form works on mobile browsers
- Test on iOS Safari and Chrome Android
- **Files**: `frontend/src/components/layout/MobileNav.tsx`, responsive CSS throughout

#### Task 8.2: Production Docker Image
- Finalize multi-stage Dockerfile
- Verify UID/GID 1000 throughout
- Add health check endpoint in Docker HEALTHCHECK
- Optimize image size (clean apt cache, remove build deps)
- Install audiowaveform in runtime image (from package or pre-built binary)
- Test: `docker build . && docker-compose up` starts cleanly
- **Files**: `Dockerfile` (finalize), `docker-compose.yml` (finalize)

#### Task 8.3: Security Hardening
- Verify all SQL uses parameterized queries (audit)
- Verify path traversal protection (SafeJoin in all file operations)
- Verify CSRF protection (SameSite=Strict + Origin header validation)
- Verify share tokens use crypto/rand (not math/rand)
- Verify file upload magic-byte validation
- Verify comment content is plain text (no HTML injection)
- Rate limit public share endpoints
- Add security headers middleware (X-Content-Type-Options, X-Frame-Options, etc.)
- **Files**: `internal/middleware/security.go`, audit of existing handlers

#### Task 8.4: Integration Testing
- API integration tests for critical flows:
  - Auth: login -> callback -> me -> logout
  - Upload: POST recording -> verify file on disk -> verify processing status
  - Playback: GET stream with Range header -> verify 206 response
  - Comments: POST comment -> GET threaded -> verify structure
  - Shares: POST share -> GET public -> verify no auth needed
  - Admin: GET dump -> verify ZIP structure
- Use docker-compose postgres for test DB
- **Files**: `internal/auth/handler_test.go`, `internal/recordings/handler_test.go`, `internal/shares/handler_test.go`, `internal/admin/handler_test.go`

---

## Dependency Graph (Critical Path)

```
Wave 1 (Scaffold) ─────────────────────────────────────────────────────────┐
  ├─ 1.1 Go scaffold ──┐                                                  │
  ├─ 1.2 React scaffold │ (parallel)                                      │
  └─ 1.3 Docker setup ──┘                                                 │
                         │                                                 │
Wave 2 (Foundation) ─────┤                                                 │
  ├─ 2.1 Config ────────┐│                                                │
  ├─ 2.2 Database ──────┤│ (parallel)                                     │
  ├─ 2.3 HTTP server ───┤│                                                │
  ├─ 2.4 Storage ───────┤│                                                │
  └─ 2.5 Frontend shell ┘│                                                │
                          │                                                │
Wave 3 (Auth + Upload) ──┤                                                 │
  ├─ 3.1 OIDC auth ─────┐│ (3.1 blocks most of Wave 4-7 frontend)        │
  ├─ 3.2 Upload endpoint ┤│ (3.2 blocks 4.2 processing pipeline)         │
  └─ 3.3 Theme system ──┘│                                                │
                          │                                                │
Wave 4 (Pipeline + APIs) ┤                                                 │
  ├─ 4.1 Auth UI (needs 3.1) ────────────────┐                            │
  ├─ 4.2 Processing pipeline (needs 3.2) ────┤ (parallel)                 │
  ├─ 4.3 Songs API ──────────────────────────┤                            │
  ├─ 4.4 Comments API ──────────────────────┤                             │
  ├─ 4.5 Tags API ──────────────────────────┤                             │
  └─ 4.6 Recording list/detail API ─────────┘                             │
                                              │                            │
Wave 5 (Streaming + Shares) ─────────────────┤                            │
  ├─ 5.1 Streaming (needs 4.2) ─────────────┐│                            │
  ├─ 5.2 Waveform endpoint (needs 4.2) ─────┤│ (parallel)                │
  ├─ 5.3 Upload UI ─────────────────────────┤│                            │
  ├─ 5.4 Library UI ────────────────────────┤│                            │
  ├─ 5.5 Tags UI ──────────────────────────┤│                             │
  ├─ 5.6 Shares backend ──────────────────┤│                              │
  └─ 5.7 Admin backend ──────────────────┘│                               │
                                           │                               │
Wave 6 (Core UI) ─────────────────────────┤                                │
  ├─ 6.1 Waveform player (needs 5.1, 5.2) ┐│                              │
  ├─ 6.2 Recording detail page ────────────┤│ (parallel)                  │
  └─ 6.3 Admin UI ────────────────────────┘│                              │
                                            │                              │
Wave 7 (Collaboration) ───────────────────┤                                │
  ├─ 7.1 Song markers UI (needs 6.1) ─────┐│                              │
  ├─ 7.2 Comments UI (needs 6.1) ─────────┤│ (parallel)                  │
  └─ 7.3 Share UI + snippet player ───────┘│                              │
                                            │                              │
Wave 8 (Polish) ──────────────────────────┘                                │
  ├─ 8.1 Mobile ───────┐                                                   │
  ├─ 8.2 Docker prod ──┤ (parallel)                                        │
  ├─ 8.3 Security ─────┤                                                   │
  └─ 8.4 Integration tests                                                 │
```

**Critical path**: 1.1 -> 2.2 -> 3.1 + 3.2 -> 4.2 -> 5.1 -> 6.1 -> 7.1/7.2 -> 8.1

---

## Risk Mitigations

| Risk | Mitigation |
|------|-----------|
| Large file uploads fail mid-stream | Stream to temp file, atomic rename on success. Orphan cleanup cron. Future: add TUS via Uppy transport swap |
| ffmpeg/audiowaveform processing hangs | Context cancellation with configurable timeout (default 5 min). Processing errors logged, recording stays in "processing" state, does not crash server |
| OIDC provider (Pocket ID) unavailable | DB-backed sessions continue working for existing users. Share links have zero OIDC dependency. Only new logins are blocked |
| Browser can't play uploaded format | AAC derivative eliminates this. Original preserved for download |
| Mobile browser crashes on large waveform | Precomputed peaks (~80KB JSON) render in <100ms. No client-side audio decoding |
| Concurrent ffmpeg jobs OOM the pod | Bounded goroutine pool (default 4 workers). Configurable via env var |
| Filesystem/DB desync (orphaned files) | All file operations are DB-first (record created before file written). Startup reconciliation: warn on files without DB records. Future: periodic cleanup job |
| Date metadata missing from uploaded files | Fallback chain: embedded tags -> filename date parsing -> upload timestamp. User can always override via PATCH |
| Accidental deletion by band members | Soft deletes only. No permanent deletion in MVP. Future: admin-only hard delete |
| Path traversal attacks | SafeJoin helper validates resolved path starts with storage root. Files served by UUID, never by user-provided filename |

---

## MVP Scope Summary

### IN (Wave 1-8)
- Multi-file upload with streaming + magic-byte validation
- Async processing: ffprobe metadata, AAC derivative, waveform peaks
- Browse by date (DB-driven), filter by tag, search by title
- Audio playback with Range requests on AAC derivative
- Waveform visualization from precomputed peaks (wavesurfer.js)
- Song markers/timestamps (start + optional end)
- Threaded comments at specific timestamps
- Tagging with color-coded badges
- Public share links with Snippet Player (token-based, no auth)
- Segment downloads (server-side ffmpeg, AAC output)
- Admin data dump (ZIP stream)
- OIDC auth via Pocket ID (Authorization Code + PKCE)
- Dark/light mode (system default + toggle)
- Full mobile support (responsive layout)
- Single Docker image, UID/GID 1000

### OUT (Post-MVP)
- Resumable TUS uploads
- MediaSession API (lock screen controls)
- Speed/pitch controls (requires pitch-invariant time stretching)
- Spatial waveform annotations (comments visually on waveform)
- Real RBAC enforcement (middleware stub in place)
- Public comments on share links
- Silence detection / auto-splitting
- S3/MinIO storage backend (interface ready)
- Separate worker container for media processing
- Segment download format selection (always AAC)
