import { forwardRef, useImperativeHandle, useRef, useEffect, useCallback } from 'react';
import { useAudioPlayer } from '@/hooks/useAudioPlayer';
import { useMediaSession, type MediaSessionMarker } from '@/hooks/useMediaSession';
import { PlayerControls } from '@/components/player/PlayerControls';
import { ProcessingStatus } from './ProcessingStatus';
import { type Recording, thumbnailUrl } from '@/api/recordings';
import { cn } from '@/lib/utils';

export interface WaveformPlayerProps {
  recording: Recording;
  audioUrlOverride?: string;
  peaksUrlOverride?: string;
  initialTime?: number;
  autoPlay?: boolean;
  onTimeUpdate?: (time: number) => void;
  onSeek?: (time: number) => void;
  markers?: MediaSessionMarker[];
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
  ({ recording, audioUrlOverride, peaksUrlOverride, initialTime, autoPlay, onTimeUpdate, onSeek, markers, showVideo, onShowVideoChange, className }, ref) => {
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
      markers,
    });

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
        />
      </div>
    );
  }
);

WaveformPlayer.displayName = 'WaveformPlayer';
