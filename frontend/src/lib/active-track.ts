export interface TrackMarker {
  title: string;
  startSeconds: number;
}

/**
 * Returns the currently active track based on playback position.
 * Assumes markers are sorted by startSeconds ascending.
 * The active track is the last marker whose startSeconds <= currentTime.
 */
export function getActiveTrack<T extends TrackMarker>(
  sortedMarkers: readonly T[],
  currentTime: number,
): T | undefined {
  for (let i = sortedMarkers.length - 1; i >= 0; i--) {
    if (currentTime >= sortedMarkers[i].startSeconds) {
      return sortedMarkers[i];
    }
  }
  return undefined;
}
