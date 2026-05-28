import { forwardRef, useImperativeHandle, useRef, useEffect, useCallback, useMemo } from 'react';
import { useVideoPlayer } from '@/hooks/useVideoPlayer';
import { useMediaSession } from '@/hooks/useMediaSession';
import { PlayerControls } from '@/components/player/PlayerControls';
import { ProcessingStatus } from '@/components/audio/ProcessingStatus';
import { type Recording, thumbnailUrl, streamUrl } from '@/api/recordings';
import { cn } from '@/lib/utils';
import type { Song } from '@/api/songs';

export interface VideoPlayerProps {
  recording: Recording;
  streamUrlOverride?: string;
  initialTime?: number;
  autoPlay?: boolean;
  onTimeUpdate?: (time: number) => void;
  onSeek?: (time: number) => void;
  songs?: Song[];
  onMarkerClick?: (startSeconds: number) => void;
  showVideo?: boolean;
  onShowVideoChange?: (show: boolean) => void;
  className?: string;
}

export interface VideoPlayerRef {
  seekTo: (seconds: number) => void;
  stop: () => void;
  getCurrentTime: () => number;
  getIsPlaying: () => boolean;
}

export const VideoPlayer = forwardRef<VideoPlayerRef, VideoPlayerProps>(
  ({ recording, streamUrlOverride, initialTime, autoPlay, onTimeUpdate, onSeek, songs, onMarkerClick, showVideo, onShowVideoChange, className }, ref) => {
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
    } = useVideoPlayer({ videoRef, initialTime, autoPlay, onTimeUpdate, onSeek });

    useImperativeHandle(ref, () => ({
      seekTo: seekToTime,
      stop: () => {
        if (videoRef.current && !videoRef.current.paused) {
          videoRef.current.pause();
        }
      },
      getCurrentTime: () => currentTime,
      getIsPlaying: () => isPlaying,
    }));

    const handleFullscreen = useCallback(() => {
      if (videoRef.current) {
        void videoRef.current.requestFullscreen();
      }
    }, []);

    const play = useCallback(() => {
      if (!isPlaying) togglePlay();
    }, [isPlaying, togglePlay]);

    const pause = useCallback(() => {
      if (isPlaying) togglePlay();
    }, [isPlaying, togglePlay]);

    const stop = useCallback(() => {
      if (videoRef.current && !videoRef.current.paused) {
        videoRef.current.pause();
      }
    }, []);

    const mediaSessionMarkers = useMemo(
      () => songs?.map((s) => ({ title: s.title, startSeconds: s.start_seconds })),
      [songs],
    );

    const sortedSongs = useMemo(
      () => (songs ? [...songs].sort((a, b) => a.start_seconds - b.start_seconds) : []),
      [songs],
    );

    useMediaSession({
      title: recording.title,
      artworkUrl: posterUrl,
      isPlaying,
      currentTime,
      duration,
      onPlay: play,
      onPause: pause,
      onSeekToTime: seekToTime,
      onStop: stop,
      markers: mediaSessionMarkers,
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
      return () => window.removeEventListener('keydown', handleKeyDown);
    }, [togglePlay, seekToTime, currentTime]);

    if (recording.processing_step !== 'complete') {
      return (
        <div className={cn('p-4 rounded-lg bg-card border shadow-sm', className)}>
          <ProcessingStatus
            processingStep={recording.processing_step}
            mediaType={recording.media_type}
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
        <PlayerControls
          isPlaying={isPlaying}
          currentTime={currentTime}
          duration={duration}
          volume={volume}
          isReady={isReady}
          onTogglePlay={togglePlay}
          onVolumeChange={setVolume}
          onSeek={seek}
          onFullscreen={handleFullscreen}
          showVideo={showVideo}
          onShowVideoChange={onShowVideoChange}
          songs={songs}
          onMarkerClick={onMarkerClick ?? seekToTime}
          onPreviousTrack={sortedSongs.length > 0 ? handlePreviousTrack : undefined}
          onNextTrack={sortedSongs.length > 0 ? handleNextTrack : undefined}
        />
      </div>
    );
  }
);

VideoPlayer.displayName = 'VideoPlayer';
