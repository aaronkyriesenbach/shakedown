# Shakedown Frontend

React + TypeScript + Vite application using shadcn/ui components and Tailwind CSS.

## Development

```sh
npm install
npm run dev
```

The Vite dev server starts on port 5173 and proxies `/api` requests to the Go backend at `http://localhost:8080` (override with `VITE_API_PROXY` env var).

## Structure

```
src/
├── api/              # React Query hooks + API types
├── components/
│   ├── audio/        # WaveformPlayer, ProcessingStatus
│   ├── video/        # VideoPlayer
│   ├── player/       # PlayerControls (shared between audio/video)
│   ├── recordings/   # RecordingCard, RecordingList, RecordingDetail
│   ├── comments/     # CommentForm, CommentThread
│   ├── songs/        # SongMarkerList
│   ├── tags/         # TagFilter, TagManager
│   ├── shares/       # ShareDialog
│   ├── auth/         # Login, user menu
│   ├── layout/       # App shell, nav
│   └── ui/           # shadcn/ui primitives (Button, Card, Dialog, etc.)
├── hooks/            # useAudioPlayer, useVideoPlayer, useMediaSession, useTheme
├── lib/              # Utilities (cn, format helpers)
└── pages/            # Route-level page components
```

## Scripts

| Command | Description |
|---|---|
| `npm run dev` | Start dev server with HMR |
| `npm run build` | Type-check + production build |
| `npm run lint` | Run ESLint |
| `npm run preview` | Preview production build locally |
