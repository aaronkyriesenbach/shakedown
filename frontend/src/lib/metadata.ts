import { parseBlob } from 'music-metadata';

export async function extractRecordingDate(file: File): Promise<string | null> {
  try {
    const metadata = await parseBlob(file, { duration: false, skipCovers: true });

    if (metadata.format.creationTime) {
      return formatDate(metadata.format.creationTime);
    }

    if (metadata.common.date) {
      const parsed = new Date(metadata.common.date);
      if (!isNaN(parsed.getTime())) {
        return formatDate(parsed);
      }
    }

    if (metadata.common.year) {
      return `${metadata.common.year}-01-01`;
    }

    return null;
  } catch {
    return null;
  }
}

function formatDate(date: Date): string {
  return date.toISOString().split('T')[0];
}
