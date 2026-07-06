CREATE TABLE cloud_sync_state (
  recording_id      UUID PRIMARY KEY REFERENCES recordings(id) ON DELETE CASCADE,
  remote_path       TEXT NOT NULL,
  remote_file_id    TEXT,
  file_size_bytes   BIGINT,
  remote_size_bytes BIGINT,
  status            TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','syncing','synced','error')),
  error             TEXT,
  error_class       TEXT,
  attempts          INT NOT NULL DEFAULT 0,
  last_attempt_at   TIMESTAMPTZ,
  next_attempt_at   TIMESTAMPTZ,
  lease_owner       TEXT,
  lease_expires_at  TIMESTAMPTZ,
  synced_at         TIMESTAMPTZ,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT uq_cloud_sync_remote_path UNIQUE (remote_path)
);
CREATE INDEX idx_cloud_sync_status ON cloud_sync_state(status);
CREATE INDEX idx_cloud_sync_status_next_attempt ON cloud_sync_state(status, next_attempt_at);
