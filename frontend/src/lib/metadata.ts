import { parseBlob } from 'music-metadata';

export interface RecordingDateInfo {
  date: string;
  timestamp: string | null;
}

export async function extractRecordingDate(file: File): Promise<string | null> {
  const info = await extractRecordingDateInfo(file);
  return info?.date ?? null;
}

export async function extractRecordingDateInfo(file: File): Promise<RecordingDateInfo | null> {
  try {
    const metadata = await parseBlob(file, { duration: false, skipCovers: true });

    if (metadata.format.creationTime) {
      return {
        date: formatDate(metadata.format.creationTime),
        timestamp: metadata.format.creationTime.toISOString(),
      };
    }

    if (metadata.common.date) {
      const parsed = new Date(metadata.common.date);
      if (!isNaN(parsed.getTime())) {
        return {
          date: formatDate(parsed),
          timestamp: parsed.toISOString(),
        };
      }
    }

    if (metadata.common.year) {
      return {
        date: `${metadata.common.year}-01-01`,
        timestamp: null,
      };
    }

    return null;
  } catch {
    return null;
  }
}

function formatDate(date: Date): string {
  return date.toISOString().split('T')[0];
}
