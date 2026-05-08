ALTER TABLE recordings
  ADD COLUMN processing_step TEXT NOT NULL DEFAULT 'queued'
    CHECK (processing_step IN ('queued','analyzing','transcoding','generating_waveform','complete'));
