import { useEffect, useRef } from 'react';

export interface MediaSessionMarker {
  title: string;
  startSeconds: number;
}

export interface UseMediaSessionProps {
  title: string;
  artist?: string;
  album?: string;
  artworkUrl?: string;
  isPlaying: boolean;
  currentTime: number;
  duration: number;
  onPlay: () => void;
  onPause: () => void;
  onSeekToTime: (seconds: number) => void;
  onStop: () => void;
  markers?: MediaSessionMarker[];
}

const SKIP_SECONDS = 10;

/**
 * Threshold in seconds — if the current position is within this many seconds
 * of a marker's start, "previous track" jumps to the marker before it rather
 * than restarting the current one.
 */
const PREV_MARKER_THRESHOLD = 2;

/**
 * Small offset added when searching for the "next" marker so that sitting
 * exactly on a marker boundary doesn't return the same marker.
 */
const NEXT_MARKER_EPSILON = 1;

export function useMediaSession({
  title,
  artist,
  album,
  artworkUrl,
  isPlaying,
  currentTime,
  duration,
  onPlay,
  onPause,
  onSeekToTime,
  onStop,
  markers,
}: UseMediaSessionProps): void {
  const onPlayRef = useRef(onPlay);
  const onPauseRef = useRef(onPause);
  const onSeekToTimeRef = useRef(onSeekToTime);
  const onStopRef = useRef(onStop);

  useEffect(() => {
    onPlayRef.current = onPlay;
    onPauseRef.current = onPause;
    onSeekToTimeRef.current = onSeekToTime;
    onStopRef.current = onStop;
  }, [onPlay, onPause, onSeekToTime, onStop]);

  const currentTimeRef = useRef(currentTime);
  const durationRef = useRef(duration);

  useEffect(() => {
    currentTimeRef.current = currentTime;
    durationRef.current = duration;
  }, [currentTime, duration]);

  useEffect(() => {
    if (!('mediaSession' in navigator)) return;

    const artwork: MediaImage[] = [];
    if (artworkUrl) {
      artwork.push({ src: artworkUrl, sizes: '512x512', type: 'image/jpeg' });
    }

    navigator.mediaSession.metadata = new MediaMetadata({
      title,
      artist: artist ?? undefined,
      album: album ?? undefined,
      artwork,
    });
  }, [title, artist, album, artworkUrl]);

  useEffect(() => {
    if (!('mediaSession' in navigator)) return;
    navigator.mediaSession.playbackState = isPlaying ? 'playing' : 'paused';
  }, [isPlaying]);

  useEffect(() => {
    if (!('mediaSession' in navigator)) return;
    if (duration <= 0) return;

    navigator.mediaSession.setPositionState({
      duration,
      playbackRate: 1,
      position: Math.min(currentTime, duration),
    });
  }, [currentTime, duration]);

  // --- Action handlers ---
  useEffect(() => {
    if (!('mediaSession' in navigator)) return;

    const sortedMarkers = markers
      ? [...markers].sort((a, b) => a.startSeconds - b.startSeconds)
      : [];
    const hasMarkers = sortedMarkers.length > 0;

    const handlePlay = () => onPlayRef.current();
    const handlePause = () => onPauseRef.current();
    const handleStop = () => onStopRef.current();

    const handleSeekTo = (details: MediaSessionActionDetails) => {
      if (details.seekTime !== undefined && details.seekTime !== null) {
        onSeekToTimeRef.current(details.seekTime);
      }
    };

    const handleSeekBackward = () => {
      onSeekToTimeRef.current(Math.max(0, currentTimeRef.current - SKIP_SECONDS));
    };

    const handleSeekForward = () => {
      onSeekToTimeRef.current(
        Math.min(durationRef.current, currentTimeRef.current + SKIP_SECONDS),
      );
    };

    navigator.mediaSession.setActionHandler('play', handlePlay);
    navigator.mediaSession.setActionHandler('pause', handlePause);
    navigator.mediaSession.setActionHandler('stop', handleStop);
    navigator.mediaSession.setActionHandler('seekto', handleSeekTo);
    navigator.mediaSession.setActionHandler('seekbackward', handleSeekBackward);
    navigator.mediaSession.setActionHandler('seekforward', handleSeekForward);

    if (hasMarkers) {
      const handlePreviousTrack = () => {
        const now = currentTimeRef.current;
        // Find the last marker that starts before (now - threshold)
        let target: MediaSessionMarker | undefined;
        for (let i = sortedMarkers.length - 1; i >= 0; i--) {
          if (sortedMarkers[i].startSeconds < now - PREV_MARKER_THRESHOLD) {
            target = sortedMarkers[i];
            break;
          }
        }
        onSeekToTimeRef.current(target ? target.startSeconds : 0);
      };

      const handleNextTrack = () => {
        const now = currentTimeRef.current;
        // Find the first marker that starts after (now + epsilon)
        const target = sortedMarkers.find(
          (m) => m.startSeconds > now + NEXT_MARKER_EPSILON,
        );
        if (target) {
          onSeekToTimeRef.current(target.startSeconds);
        }
      };

      navigator.mediaSession.setActionHandler('previoustrack', handlePreviousTrack);
      navigator.mediaSession.setActionHandler('nexttrack', handleNextTrack);
    } else {
      navigator.mediaSession.setActionHandler('previoustrack', null);
      navigator.mediaSession.setActionHandler('nexttrack', null);
    }

    return () => {
      navigator.mediaSession.setActionHandler('play', null);
      navigator.mediaSession.setActionHandler('pause', null);
      navigator.mediaSession.setActionHandler('stop', null);
      navigator.mediaSession.setActionHandler('seekto', null);
      navigator.mediaSession.setActionHandler('seekbackward', null);
      navigator.mediaSession.setActionHandler('seekforward', null);
      navigator.mediaSession.setActionHandler('previoustrack', null);
      navigator.mediaSession.setActionHandler('nexttrack', null);
    };
  }, [markers]);
}
