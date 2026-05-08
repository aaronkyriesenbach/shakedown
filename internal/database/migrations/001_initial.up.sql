CREATE EXTENSION IF NOT EXISTS "pgcrypto";

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

CREATE TABLE recordings (
  id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  title               TEXT NOT NULL,
  original_filename   TEXT NOT NULL,
  file_ext            TEXT NOT NULL,
  file_size_bytes     BIGINT NOT NULL,
  mime_type           TEXT NOT NULL,
  storage_path        TEXT NOT NULL,
  uploaded_by         UUID NOT NULL REFERENCES users(id),
  recorded_at         TIMESTAMPTZ NOT NULL,
  recorded_at_source  TEXT NOT NULL DEFAULT 'upload_timestamp' CHECK (recorded_at_source IN ('embedded_tags','filename_date','upload_timestamp','user_set')),
  duration_seconds    DOUBLE PRECISION,
  bitrate             INTEGER,
  sample_rate         INTEGER,
  channels            INTEGER,
  playback_ready      BOOLEAN NOT NULL DEFAULT false,
  waveform_ready      BOOLEAN NOT NULL DEFAULT false,
  processing_error    TEXT,
  created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at          TIMESTAMPTZ
);
CREATE INDEX idx_recordings_uploaded_by ON recordings(uploaded_by);
CREATE INDEX idx_recordings_recorded_at ON recordings(recorded_at DESC);
CREATE INDEX idx_recordings_deleted_at ON recordings(deleted_at) WHERE deleted_at IS NULL;
ALTER TABLE recordings ADD COLUMN search_vector tsvector GENERATED ALWAYS AS (to_tsvector('english', title)) STORED;
CREATE INDEX idx_recordings_search ON recordings USING GIN(search_vector);

CREATE TABLE tags (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name       TEXT NOT NULL UNIQUE,
  color      TEXT NOT NULL DEFAULT '#6366f1',
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

CREATE TABLE songs (
  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  recording_id     UUID NOT NULL REFERENCES recordings(id) ON DELETE CASCADE,
  title            TEXT NOT NULL,
  start_seconds    INTEGER NOT NULL,
  end_seconds      INTEGER,
  notes            TEXT,
  created_by       UUID NOT NULL REFERENCES users(id),
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_songs_recording ON songs(recording_id, start_seconds);

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

CREATE TABLE shares (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  token           TEXT NOT NULL UNIQUE,
  recording_id    UUID NOT NULL REFERENCES recordings(id) ON DELETE CASCADE,
  song_id         UUID REFERENCES songs(id) ON DELETE SET NULL,
  start_seconds   INTEGER,
  end_seconds     INTEGER,
  label           TEXT,
  created_by      UUID NOT NULL REFERENCES users(id),
  expires_at      TIMESTAMPTZ,
  access_count    INTEGER NOT NULL DEFAULT 0,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE sessions (
  id         TEXT PRIMARY KEY,
  user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  data       JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX idx_sessions_user ON sessions(user_id);
CREATE INDEX idx_sessions_expires ON sessions(expires_at);
