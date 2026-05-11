ALTER TABLE recordings DROP COLUMN audio_extract_ready;

ALTER TABLE recordings DROP CONSTRAINT recordings_processing_step_check;
ALTER TABLE recordings ADD CONSTRAINT recordings_processing_step_check
  CHECK (processing_step IN ('queued','analyzing','transcoding','extracting_thumbnail','generating_waveform','complete'));
