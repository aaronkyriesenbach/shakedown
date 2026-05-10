import { forwardRef, useImperativeHandle, useRef, useEffect, useCallback } from 'react';
import { useVideoPlayer } from '@/hooks/useVideoPlayer';
import { VideoControls } from './VideoControls';
import { ProcessingStatus } from '@/components/audio/ProcessingStatus';
import { type Recording, thumbnailUrl, streamUrl } from '@/api/recordings';
import { cn } from '@/lib/utils';
import type { Song } from '@/api/songs';

export interface VideoPlayerProps {
  recording: Recording;
  streamUrlOverride?: string;
  onTimeUpdate?: (time: number) => void;
  onSeek?: (time: number) => void;
  songs?: Song[];
  className?: string;
}

export interface VideoPlayerRef {
  seekTo: (seconds: number) => void;
}

export const VideoPlayer = forwardRef<VideoPlayerRef, VideoPlayerProps>(
  ({ recording, streamUrlOverride, onTimeUpdate, onSeek, songs, className }, ref) => {
    const videoRef = useRef<HTMLVideoElement>(null);
    const videoUrl = streamUrlOverride ?? streamUrl(recording.id);
    const posterUrl = recording.thumbnail_ready ? thumbnailUrl(recording.id) : undefined;

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
    } = useVideoPlayer({ videoRef, onTimeUpdate, onSeek });

    useImperativeHandle(ref, () => ({
      seekTo: seekToTime,
    }));

    const handleFullscreen = useCallback(() => {
      if (videoRef.current) {
        void videoRef.current.requestFullscreen();
      }
    }, []);

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
      return () => window.removeEventListener('keydown', handleKeyDown);
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
        <video
          ref={videoRef}
          src={videoUrl}
          poster={posterUrl}
          className="w-full rounded-md bg-black aspect-video"
          preload="metadata"
          playsInline
        />
        <VideoControls
          isPlaying={isPlaying}
          currentTime={currentTime}
          duration={duration}
          volume={volume}
          isReady={isReady}
          onTogglePlay={togglePlay}
          onVolumeChange={setVolume}
          onSeek={seek}
          onFullscreen={handleFullscreen}
          songs={songs}
          onMarkerClick={seekToTime}
        />
      </div>
    );
  }
);

VideoPlayer.displayName = 'VideoPlayer';
