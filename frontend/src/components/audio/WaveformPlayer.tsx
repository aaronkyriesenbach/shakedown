import { forwardRef, useImperativeHandle, useRef, useEffect } from 'react';
import { useAudioPlayer } from '@/hooks/useAudioPlayer';
import { AudioControls } from './AudioControls';
import { ProcessingStatus } from './ProcessingStatus';
import { type Recording } from '@/api/recordings';
import { cn } from '@/lib/utils';

export interface WaveformPlayerProps {
  recording: Recording;
  audioUrlOverride?: string;
  peaksUrlOverride?: string;
  onTimeUpdate?: (time: number) => void;
  onSeek?: (time: number) => void;
  className?: string;
}

export interface WaveformPlayerRef {
  seekTo: (seconds: number) => void;
}

export const WaveformPlayer = forwardRef<WaveformPlayerRef, WaveformPlayerProps>(
  ({ recording, audioUrlOverride, peaksUrlOverride, onTimeUpdate, onSeek, className }, ref) => {
    const containerRef = useRef<HTMLDivElement>(null);
    const audioUrl = audioUrlOverride || `/api/recordings/${recording.id}/stream`;
    const peaksUrl = peaksUrlOverride !== undefined ? peaksUrlOverride : (recording.waveform_ready ? `/api/recordings/${recording.id}/waveform` : undefined);

    const {
      isPlaying,
      currentTime,
      duration,
      isReady,
      volume,
      seek,
      seekToTime,
      togglePlay,
      setVolume,
    } = useAudioPlayer({
      containerRef,
      audioUrl,
      peaksUrl,
      duration: recording.duration_seconds,
      onTimeUpdate,
      onSeek,
    });

    useImperativeHandle(ref, () => ({
      seekTo: seekToTime,
    }));

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
        <AudioControls
          isPlaying={isPlaying}
          currentTime={currentTime}
          duration={duration}
          volume={volume}
          isReady={isReady}
          onTogglePlay={togglePlay}
          onVolumeChange={setVolume}
          onSeek={seek}
        />
      </div>
    );
  }
);

WaveformPlayer.displayName = 'WaveformPlayer';
