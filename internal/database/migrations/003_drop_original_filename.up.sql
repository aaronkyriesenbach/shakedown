-- Backfill any recordings that have an empty title with a generated default.
-- Uses row_number() to assign a sequential number matching creation order.
UPDATE recordings SET title = 'Recording #' || sub.rn || ' ' || to_char(recorded_at, 'YYYY-MM-DD')
FROM (
  SELECT id, row_number() OVER (ORDER BY created_at) AS rn
  FROM recordings
  WHERE title = original_filename OR title = ''
) sub
WHERE recordings.id = sub.id;

ALTER TABLE recordings DROP COLUMN original_filename;
