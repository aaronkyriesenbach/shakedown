ALTER TABLE recordings
  ADD COLUMN media_type TEXT NOT NULL DEFAULT 'audio' CHECK (media_type IN ('audio','video'));

ALTER TABLE recordings
  ADD COLUMN thumbnail_ready BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE recordings
  ADD COLUMN video_width INTEGER;

ALTER TABLE recordings
  ADD COLUMN video_height INTEGER;

-- Replace processing_step CHECK constraint to include extracting_thumbnail
ALTER TABLE recordings DROP CONSTRAINT recordings_processing_step_check;

ALTER TABLE recordings ADD CONSTRAINT recordings_processing_step_check
  CHECK (processing_step IN ('queued','analyzing','transcoding','extracting_thumbnail','generating_waveform','complete'));
