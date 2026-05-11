ALTER TABLE recordings ADD COLUMN audio_extract_ready BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE recordings DROP CONSTRAINT recordings_processing_step_check;
ALTER TABLE recordings ADD CONSTRAINT recordings_processing_step_check
  CHECK (processing_step IN ('queued','analyzing','transcoding','extracting_thumbnail','extracting_audio','generating_waveform','complete'));
