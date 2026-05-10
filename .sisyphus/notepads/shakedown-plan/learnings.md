# Learnings

## Project Overview
Shakedown is an audio review app for bands. Key features:
- Multi-file audio upload with async processing (ffprobe, ffmpeg, audiowaveform)
- Browse/filter by date, tags, songs
- Threaded comments at timestamps
- Waveform player (wavesurfer.js v7 + BBC audiowaveform precomputed peaks)
- Public share links (token-based, no auth)
- Admin data dump (ZIP stream)
- OIDC auth via Pocket ID (Authorization Code + PKCE)
- Dark/light mode, full mobile support

## Tech Stack
- Backend: Go 1.22+ / chi router / pgx v5 / golang-migrate
- Frontend: React 18 + Vite + TypeScript (strict) + TanStack Query + shadcn/ui + Tailwind CSS
- Database: PostgreSQL 16
- Audio: ffmpeg + ffprobe + BBC audiowaveform binary
- Waveform: wavesurfer.js v7
- Upload UX: Uppy (XHR, not TUS)
- Auth: Pocket ID (OIDC) / coreos/go-oidc/v3
- Deployment: Single multi-stage Docker image (debian-slim), UID/GID 1000

## Storage Layout
/data/audio/{recording_id}/original.{ext} - immutable original
/data/audio/{recording_id}/playback.m4a - AAC 192kbps derivative
/data/audio/{recording_id}/waveform.json - BBC audiowaveform peaks

## Key Decisions
- Sessions are DB-backed (not JWT) for multi-replica readiness
- Soft deletes only (no hard delete in MVP)
- Bounded goroutine pool for processing (default 4 workers)
- Upload: stream to temp, atomic rename on success (never buffer in memory)
- File operations: DB-first (record created before file written)
- Frontend uses path aliases @/ for all imports

## Wave 1.1: Go Backend Scaffold (completed)

### Module setup
- Module name: `shakedown`, Go 1.26.2 (toolchain auto-selected, ≥1.22 requirement met)
- Chi v5 router wired in `cmd/server/main.go` with `middleware.RequestID`, `RealIP`, `Recoverer`
- Health endpoint: `GET /api/health` → `{"status":"ok","version":"dev"}`
- Server port defaults to 8080, overridable via `PORT` env var

### Dependencies added (all pinned via go.sum)
- github.com/go-chi/chi/v5 v5.2.5
- github.com/go-chi/cors v1.2.2
- github.com/jackc/pgx/v5 v5.9.2
- github.com/golang-migrate/migrate/v4 v4.19.1
- github.com/coreos/go-oidc/v3 v3.18.0
- golang.org/x/oauth2 v0.36.0
- github.com/kelseyhightower/envconfig v1.4.0
- go.uber.org/zap v1.28.0
- github.com/google/uuid v1.6.0

### Conventions established
- All stub files: bare `package <name>` declaration only, no imports, no logic
- Makefile: lint target checks for `golangci-lint`, falls back to `go vet ./...`
 - `go build ./...` and `go test ./...` both pass clean from project root
 - Wave 1.2 Scaffold frontend completed.

## Wave 8.2: Docker / Compose hardening

- Added version and commit build-time variables to cmd/server/main.go and exposed them in /api/health
- Updated Dockerfile to embed VERSION and COMMIT via build args and added -ldflags to strip debug symbols
- Declared /data as a VOLUME and ensured it's owned by UID/GID 1000
- Verified apt-get uses --no-install-recommends and cleans apt lists in same layer
- Added docker-compose.yml improvements: user mapping, logging rotation, explicit environment entries, and audiodata named volume
- Added docker-compose.prod.yml to remove DB port exposure and set app restart to always

## Wave 2.2: Database Module (completed)

### Dependencies added
- github.com/jackc/pgx/v5 v5.9.2 (pgxpool)
- github.com/golang-migrate/migrate/v4 v4.19.1
- github.com/golang-migrate/migrate/v4/database/pgx/v5 (driver)
- github.com/golang-migrate/migrate/v4/source/iofs (embed.FS source)

### Gotcha: go mod tidy ordering
`go get` then immediately `go mod tidy` removes deps if the importing .go file doesn't yet have real imports (was just `package database`). Always write the .go file first, then run `go get` + `go mod tidy`.

### Migration driver note
The golang-migrate pgx/v5 driver uses the `pgx5://` URL scheme internally. Pass the standard postgres:// DSN — the driver handles the scheme rewrite.

### embed.FS directive
`//go:embed migrations/*.sql` is a Go compiler directive (not a comment). It is non-negotiable — without it the embed.FS is empty at runtime.

## Wave 2.3: HTTP Server Setup (completed)

### Files created/modified
- `internal/static/dist/placeholder` — non-hidden placeholder so `//go:embed dist` compiles without real frontend build
- `internal/static/static.go` — embed.FS wrapper returning http.FileSystem via fs.Sub
- `internal/middleware/logging.go` — zap-based chi middleware using WrapResponseWriter for status capture
- `cmd/server/main.go` — full main() with config, DB connect (non-fatal), chi router, graceful shutdown

### Gotchas
- Go embed excludes files starting with `.` or `_` by default; `.gitkeep` won't satisfy the embed directive — use a non-hidden placeholder file or `all:dist` prefix
- `WriteTimeout: 0` intentional for streaming endpoints (uploads, audio playback)
- DB connection failure on startup is `logger.Warn`, not `logger.Fatal` — sessions may already exist
- chi subrouter alias import needed: `chimiddleware` for chi's middleware, `apimiddleware` for our internal middleware package

## Wave 2.5 Learnings
- Vite + TypeScript 5.8 with `erasableSyntaxOnly` requires explicit class property declarations (`public readonly status: number;` instead of constructor assignments).
- Mobile responsive layout is easier when separating `Header` entirely from `AppLayout` wrapper instead of nesting it twice. Kept `Sheet` toggle inside the main `Header` to clean up component tree.

## Wave 3.1: OIDC Auth Backend (completed)

### Dependencies added
- github.com/coreos/go-oidc/v3 v3.18.0 (was already indirect via oauth2, promoted to direct)
- golang.org/x/oauth2 v0.36.0 (was already indirect, upgraded and promoted)
- github.com/google/uuid v1.6.0 (indirect only — not directly imported in auth package)

### Files created
- `internal/auth/oidc.go` — Provider struct, NewProvider, AuthCodeURL, Exchange, GenerateRandomString (crypto/rand)
- `internal/auth/session.go` — User, Session types; UpsertUser, CreateSession, GetSession, DeleteSession, GetUserByID
- `internal/auth/middleware.go` — RequireAuth (cookie-based), RequireAdmin, UserFromContext; contextKey typed string
- `internal/auth/handler.go` — Handler struct, NewHandler, Routes (chi.Router), login/callback/logout/me handlers

### Key patterns
- `uuid` package not directly imported — session IDs generated via `GenerateRandomString(32)` (base64 URL encoded)
- Auth provider initialization is graceful: if OIDC unavailable at startup, `authHandler` is nil and auth routes are skipped
- `/me` protected by RequireAuth inside `Routes()` method itself — no wiring needed in main.go
- State + nonce stored in short-lived (10min) HttpOnly cookies — NOT in DB
- Session cookie: HttpOnly, Secure, SameSite=Strict, 30-day expiry
- OIDC state/nonce cookies: SameSite=Lax (required for redirect flow)
- `GetSession` returns (nil, nil) on not-found/expired — nilerr linting suppressed intentionally

## Auth implementation learnings (Wave 4.1)
- Destructuring unused variables (like `user` in AuthGuard) causes TS errors in strict mode, ensure unused variables are removed or preceded with underscore (depending on tsconfig).
- DropdownMenu and standard ShadCN components are available.
- `useMe` uses `staleTime` and no `retry` to gracefully handle expected 401s when a user is not authenticated.

## Wave 4.6: Recording List + Detail API (completed)

### Repository patterns
- ListFilter / ListResult structs for paginated queries
- Dynamic WHERE clause built with conditions []string + args []interface{} + argIdx counter
- Count query reuses same WHERE args; data query appends LIMIT/OFFSET as final args
- `strings.Join(conditions, " AND ")` for WHERE clause assembly
- Empty result: initialize recs = []*Recording{} if nil to guarantee JSON `[]` not `null`
- COALESCE pattern for PATCH: `COALESCE($2, title)` only updates non-nil fields

### Handler patterns  
- Routes signature now accepts structural interfaces for songHandler/commentHandler/tagHandler
- Subrouters mounted via `r.Route("/songs", func(r chi.Router) { songHandler.Routes(r) })`
- RecordingTagRoutes (not Routes) used for the /tags subroute under recordings
- URL param changed from `{id}` to `{recordingID}` across get/update/delete handlers

### main.go patterns
- All domain handlers initialized in single `if db != nil` block
- Tags top-level route: `r.With(auth.RequireAuth(db)).Route("/tags", ...)`
- Songs/comments/tags passed as structural interface values to recHandler.Routes()

### Wave 5.3: Uppy & API Modules
- Uppy packages in v4/v5 changed their CSS export paths. Instead of `@uppy/core/dist/style.min.css`, they export as `@uppy/core/css/style.min.css`.
- The `@uppy/react` module exports components via the `exports` map, requiring paths like `@uppy/react/dashboard` for the Dashboard component rather than destructuring from the root import.
- When setting `erasableSyntaxOnly: true` in TS 5.8+, we can't use implicit class fields, and modules need isolated type definitions, but for our functional component setup, it works cleanly.

## Wave 5.5: Tags UI
- Built `TagManager.tsx` with a preset color palette array that maps predefined colors to tag objects.
- To display colors aesthetically and contrast properly, used inline style variations for badges (e.g. background with 20% opacity using hex padding `20` like `${tag.color}20` and border matching the exact color or similar).
- `RecordingEditDialog.tsx` handles complex state (title, recordedAt date parsing to `YYYY-MM-DD` and back to `toISOString`) seamlessly with TanStack query.
- Use `useEffect` to sync the modal's internal form state with the selected props to avoid stale data when re-opening a dialog for a different row.
- Ensured absolute type-safe imports with `type { Recording }` when `verbatimModuleSyntax` is enabled in `tsconfig.json`.

## Library/Browse UI Implementation
- Extracted generic duration and file size formatting logic to `lib/format.ts` to keep `lib/utils.ts` cleanly focused on CSS utility functions.
- Used type intersections (e.g. `export type RecordingWithTags = Recording & { tags?: Tag[] }`) to type the UI representation of a Recording with its tags, rather than altering the strictly auto-generated API types. This ensures "no any types" are used while remaining flexible to backend/frontend differences in schema relationships.
- Debounced search state using a 300ms `setTimeout` in a React `useEffect`, pushing changes to `ListFilter` only when typing stops.
- Rendered fallback UI with `lucide-react` icons (like `Music` or `RefreshCw` animated spin) for specific status fields (`processing_error`, `playback_ready`, `waveform_ready`).
- wavesurfer.js v7 requires peaks as `number[][]` (array of arrays per channel) rather than just a flat array when passed in the  property.
- wavesurfer.js v7 requires peaks as `number[][]` (array of arrays per channel) rather than just a flat array when passed in the `peaks` property.

## Admin UI
- Created `frontend/src/api/admin.ts` to manage admin-specific API endpoints (users, download dump) separate from standard auth.
- Implemented `AdminDump` to handle file downloads elegantly via an `<a>` tag with a dynamically calculated URL `href="/api/admin/dump"` directly.
- Implemented `UserManagement` table showing admin role toggle mutations, efficiently preventing current user from downgrading their own privileges by integrating with `@/api/auth`'s `useMe` hook.
- Added visual warnings in `AdminPage` context to remind users of global scope of admin mutations, reinforcing safety limits.

### Wave 6.2: Recording Detail Page
- The `WaveformPlayer` component internally handles the display of `AudioControls` and handles the waveform loading correctly. Passing the `recording` prop is enough to power it up.
- shadcn/ui components (`Card`, `Badge`, `Tabs`, `Dialog`) make for a quick and standardized UI development.
- Always check exported types in neighboring files before declaring duplicate intersection types (e.g. `RecordingWithTags` was exported in `RecordingCard.tsx`, though redefining it locally reduces coupling).

## Wave 7.2: Comments UI
- Added `formatRelativeTime` to `lib/format.ts` for "just now", "X minutes ago" formatting
- Created `CommentThread` for displaying a tree of comments and replies
- Created `CommentForm` with timestamp support for inline video-like commenting
- Integrated into `RecordingDetail.tsx` under the "comments" tab
- Ensured typing and TanStack Query hooks were correct for tree structured comments with standard `onSuccess` cache invalidations

## Wave 7.3: Share UI & Snippet Player
- When implementing public share pages that use components normally dependent on authenticated endpoints (like WaveformPlayer fetching peaks), ensure you provide mechanisms to disable or override those features (e.g., passing `peaksUrlOverride={undefined}` or empty strings to bypass authenticated API calls).
- Used `audioUrlOverride` pattern in WaveformPlayer to allow reusing the component for public `/api/s/{token}/stream` endpoints without changing its core behavior.

- Used React Query invalidateQueries with specific recordingId to refresh song markers list
- Kept UI consistent with the app's dark music studio aesthetic
- Linked WaveformPlayer seeking with SongMarkerList row clicks via exposed ref
- Passed currentTime from WaveformPlayer onTimeUpdate directly to SongMarkerList to display the active marker state

## Wave 8.3: Security Hardening (completed)

### Security headers middleware
- `internal/middleware/security.go` — `SecurityHeaders(next http.Handler) http.Handler` (no config, direct handler wrap)
- Mounted via `r.Use(apimiddleware.SecurityHeaders)` after Logger in `cmd/server/main.go`
- Headers set: X-Content-Type-Options, X-Frame-Options, X-XSS-Protection, Referrer-Policy, Content-Security-Policy
- CSP uses `unsafe-inline` for style-src (Tailwind runtime classes); `blob:` + `data:` for media/img (waveform/audio); `frame-ancestors 'none'`

### Security audit results (all PASS)
- **Path traversal**: `LocalStorage.SafeJoin` in `recordings/storage.go` uses `filepath.Clean("/"+subPath)` and prefix-checks against `root+separator` — robust.
- **SQL injection**: pgx/v5 parameterized queries (`$1, $2, …`) used throughout; no string-interpolated SQL found.
- **CSRF**: `shakedown_session` cookie uses `SameSite=Strict` (`auth/handler.go:129`). OIDC state compared cookie→query in callback. `oidc_state`/`oidc_nonce` use `SameSite=Lax` (correct for cross-site redirect flow).
- **Share tokens**: `crypto/rand` 32-byte tokens, base64url-encoded (`shares/repository.go`).
- **Magic bytes**: `ValidateAudioMagicBytes` inspects first 32 bytes before accepting any upload (`recordings/validation.go`).
- **Comment XSS**: Plain TEXT in DB; React JSX escapes on render.
- **Upload size**: `http.MaxBytesReader` applied before `ParseMultipartForm` (`recordings/handler.go:66`).

### Mobile Optimization & Responsive Layouts
- Replaced hamburger menu `Sheet` with a bottom `MobileNav` for mobile screens (`< sm`) while keeping standard sidebar for larger screens (`sm` and up).
- Used `pb-safe` to handle iOS safe area inset (bottom) for fixed bottom navigation. 
- Integrated touch support for waveforms on mobile by adding `touch-none` to the container and allowing WaveSurfer's `height` to be responsive using `'auto'` while relying on Tailwind classes (`h-[60px] sm:h-[80px]`) for actual sizing.
- Added responsive wraps to filtering controls (`flex-col sm:flex-row`) ensuring they remain accessible without overflowing on mobile devices.

## Wave 8.4: Integration Testing (completed)

### Go httptest testing patterns used

**Internal (white-box) test packages** (`package auth`, `package shares`, `package recordings`) give access to unexported symbols and methods. This was required to:
- Call unexported handler methods (`h.me`, `h.waveformData`, etc.)
- Inject values using unexported context keys (`userContextKey`, `shareKey`)
- Construct internal structs with nil fields for partial testing

**Context injection pattern** for auth: since `auth.contextKey` is unexported, user context can only be injected from within the `auth` package itself. External packages (like `shares`) cannot inject `auth.User` without an exported helper function — this is a design constraint worth noting for future refactoring.

**Shares/recordings mocking constraints**: `Repository` and `Service` are concrete structs with unexported `*pgxpool.Pool` fields. Without repository interfaces, handler tests that exercise the DB layer must use `t.Skip`. Refactoring to accept interfaces would enable in-process mocking.

**chi URL param injection in tests**:
```go
rctx := chi.NewRouteContext()
rctx.URLParams.Add("recordingID", "test-id")
r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
```

**health handler in main package**: `cmd/server/main.go`'s `healthHandler` is unexported and in `package main` (not importable). Contract tests must maintain a local copy in the test file.

### Test coverage achieved
- `internal/auth`: HealthHandler (pass), MeHandler/401 (pass), MeHandler/200 (pass)
- `internal/shares`: GetShare/404 (pass), GetShare/200 (pass), CreateShare/401 (pass)
- `internal/recordings`: NewHandler construction (pass), Routes registration (pass)
- Skipped with clear reasons: waveform/stream endpoints (need repository interface), CreateShare with user injection (need auth helper export)
