import { forwardRef, useImperativeHandle, useRef, useEffect, useCallback, useMemo } from 'react';
import { useAudioPlayer } from '@/hooks/useAudioPlayer';
import { useMediaSession, type MediaSessionMarker } from '@/hooks/useMediaSession';
import { PlayerControls } from '@/components/player/PlayerControls';
import { ProcessingStatus } from './ProcessingStatus';
import { type Recording, thumbnailUrl } from '@/api/recordings';
import { cn } from '@/lib/utils';
import type { Song } from '@/api/songs';

export interface WaveformPlayerProps {
  recording: Recording;
  audioUrlOverride?: string;
  peaksUrlOverride?: string;
  initialTime?: number;
  autoPlay?: boolean;
  onTimeUpdate?: (time: number) => void;
  onSeek?: (time: number) => void;
  markers?: MediaSessionMarker[];
  songs?: Song[];
  showVideo?: boolean;
  onShowVideoChange?: (show: boolean) => void;
  className?: string;
}

export interface WaveformPlayerRef {
  seekTo: (seconds: number) => void;
  stop: () => void;
  getCurrentTime: () => number;
  getIsPlaying: () => boolean;
}

export const WaveformPlayer = forwardRef<WaveformPlayerRef, WaveformPlayerProps>(
  ({ recording, audioUrlOverride, peaksUrlOverride, initialTime, autoPlay, onTimeUpdate, onSeek, markers, songs, showVideo, onShowVideoChange, className }, ref) => {
    const containerRef = useRef<HTMLDivElement>(null);
    const audioUrl = audioUrlOverride || `/api/recordings/${recording.id}/stream`;
    const peaksUrl = peaksUrlOverride !== undefined ? peaksUrlOverride : (recording.waveform_ready ? `/api/recordings/${recording.id}/waveform` : undefined);

    const {
      isPlaying,
      currentTime,
      duration,
      isReady,
      volume,
      seekToTime,
      togglePlay,
      setVolume,
      stop,
    } = useAudioPlayer({
      containerRef,
      audioUrl,
      peaksUrl,
      duration: recording.duration_seconds,
      initialTime,
      autoPlay,
      onTimeUpdate,
      onSeek,
    });

    useImperativeHandle(ref, () => ({
      seekTo: seekToTime,
      stop,
      getCurrentTime: () => currentTime,
      getIsPlaying: () => isPlaying,
    }));

    const play = useCallback(() => {
      if (!isPlaying) togglePlay();
    }, [isPlaying, togglePlay]);

    const pause = useCallback(() => {
      if (isPlaying) togglePlay();
    }, [isPlaying, togglePlay]);

    const artworkUrl = recording.thumbnail_ready ? thumbnailUrl(recording.id) : undefined;

    const sortedSongs = useMemo(
      () => (songs ? [...songs].sort((a, b) => a.start_seconds - b.start_seconds) : []),
      [songs],
    );

    const mediaSessionMarkers = useMemo(
      () => markers ?? sortedSongs.map((s) => ({ title: s.title, startSeconds: s.start_seconds })),
      [markers, sortedSongs],
    );

    useMediaSession({
      title: recording.title,
      artworkUrl,
      isPlaying,
      currentTime,
      duration,
      onPlay: play,
      onPause: pause,
      onSeekToTime: seekToTime,
      onStop: stop,
      markers: mediaSessionMarkers.length > 0 ? mediaSessionMarkers : undefined,
    });

    const handlePreviousTrack = useCallback(() => {
      if (sortedSongs.length === 0) return;
      let target: Song | undefined;
      for (let i = sortedSongs.length - 1; i >= 0; i--) {
        if (sortedSongs[i].start_seconds < currentTime - 2) {
          target = sortedSongs[i];
          break;
        }
      }
      seekToTime(target ? target.start_seconds : 0);
    }, [sortedSongs, currentTime, seekToTime]);

    const handleNextTrack = useCallback(() => {
      if (sortedSongs.length === 0) return;
      const target = sortedSongs.find((s) => s.start_seconds > currentTime + 1);
      if (target) seekToTime(target.start_seconds);
    }, [sortedSongs, currentTime, seekToTime]);

    useEffect(() => {
      const handleKeyDown = (e: KeyboardEvent) => {
        if (e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement) {
          return;
        }

        switch (e.code) {
          case 'Space':
            e.preventDefault();
            togglePlay();
            break;
          case 'ArrowLeft':
            e.preventDefault();
            seekToTime(currentTime - 5);
            break;
          case 'ArrowRight':
            e.preventDefault();
            seekToTime(currentTime + 5);
            break;
        }
      };

      window.addEventListener('keydown', handleKeyDown);
      return () => {
        window.removeEventListener('keydown', handleKeyDown);
      };
    }, [togglePlay, seekToTime, currentTime]);

    if (recording.processing_step !== 'complete') {
      return (
        <div className={cn('p-4 rounded-lg bg-card border shadow-sm', className)}>
          <ProcessingStatus
            processingStep={recording.processing_step}
            processingError={recording.processing_error}
          />
        </div>
      );
    }

    return (
      <div className={cn('p-4 rounded-lg bg-card border shadow-sm flex flex-col gap-4', className)}>
        <div ref={containerRef} className="w-full h-[60px] sm:h-[80px] bg-muted/20 rounded overflow-hidden touch-none" />
        <PlayerControls
          isPlaying={isPlaying}
          currentTime={currentTime}
          duration={duration}
          volume={volume}
          isReady={isReady}
          onTogglePlay={togglePlay}
          onVolumeChange={setVolume}
          showVideo={showVideo}
          onShowVideoChange={onShowVideoChange}
          onPreviousTrack={sortedSongs.length > 0 ? handlePreviousTrack : undefined}
          onNextTrack={sortedSongs.length > 0 ? handleNextTrack : undefined}
        />
      </div>
    );
  }
);

WaveformPlayer.displayName = 'WaveformPlayer';
