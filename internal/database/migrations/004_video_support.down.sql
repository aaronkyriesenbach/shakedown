-- Remove video support columns
ALTER TABLE recordings DROP COLUMN IF EXISTS media_type;
ALTER TABLE recordings DROP COLUMN IF EXISTS thumbnail_ready;
ALTER TABLE recordings DROP COLUMN IF EXISTS video_width;
ALTER TABLE recordings DROP COLUMN IF EXISTS video_height;

-- Restore original processing_step constraint values
ALTER TABLE recordings DROP CONSTRAINT recordings_processing_step_check;

ALTER TABLE recordings ADD CONSTRAINT recordings_processing_step_check
  CHECK (processing_step IN ('queued','analyzing','transcoding','generating_waveform','complete'));
