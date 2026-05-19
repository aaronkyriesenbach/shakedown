import { useState, useCallback, useEffect, useRef, type RefObject } from 'react';
import { safeSeek } from '@/lib/media';

export interface UseVideoPlayerProps {
  videoRef: RefObject<HTMLVideoElement | null>;
  initialTime?: number;
  autoPlay?: boolean;
  onTimeUpdate?: (time: number) => void;
  onSeek?: (time: number) => void;
}

export interface UseVideoPlayerReturn {
  isPlaying: boolean;
  currentTime: number;
  duration: number;
  isReady: boolean;
  volume: number;
  togglePlay: () => void;
  seekToTime: (seconds: number) => void;
  seek: (fraction: number) => void;
  setVolume: (v: number) => void;
}

export function useVideoPlayer({ videoRef, initialTime, autoPlay, onTimeUpdate, onSeek }: UseVideoPlayerProps): UseVideoPlayerReturn {
  const [isPlaying, setIsPlaying] = useState(false);
  const [currentTime, setCurrentTime] = useState(0);
  const [duration, setDuration] = useState(0);
  const [isReady, setIsReady] = useState(false);
  const [volume, setVolumeState] = useState(1);
  const pendingSeekRef = useRef<number | null>(null);
  const isWarmedUpRef = useRef(false);

  const togglePlay = useCallback(() => {
    const video = videoRef.current;
    if (!video) return;

    if (!video.paused) {
      video.pause();
      return;
    }

    if (pendingSeekRef.current !== null) {
      const target = pendingSeekRef.current;
      pendingSeekRef.current = null;
      video.currentTime = target;
      void video.play();
    } else {
      void video.play();
    }
  }, [videoRef]);

  const seekToTime = useCallback((seconds: number) => {
    const video = videoRef.current;
    if (!video) return;

    if (!isWarmedUpRef.current && video.paused) {
      const prevVolume = video.volume;
      video.volume = 0;
      void video.play().catch(() => {});
      video.pause();
      video.volume = prevVolume;
      isWarmedUpRef.current = true;
    }

    const clamped = Math.max(0, Math.min(seconds, video.duration || 0));
    safeSeek(video, clamped);
    onSeek?.(clamped);
    if (video.paused) {
      pendingSeekRef.current = clamped;
    }
  }, [videoRef, onSeek]);

  const seek = useCallback((fraction: number) => {
    const video = videoRef.current;
    if (!video) return;
    seekToTime(fraction * (video.duration || 0));
  }, [videoRef, seekToTime]);

  const setVolume = useCallback((v: number) => {
    setVolumeState(v);
    if (videoRef.current) {
      videoRef.current.volume = v;
    }
  }, [videoRef]);

  useEffect(() => {
    const video = videoRef.current;
    if (!video) return;

    const handleLoadedMetadata = () => {
      setDuration(video.duration);
      setIsReady(true);
      if (initialTime && initialTime > 0) {
        safeSeek(video, Math.min(initialTime, video.duration));
      }
      if (autoPlay) {
        void video.play();
      }
    };
    const handleTimeUpdate = () => {
      setCurrentTime(video.currentTime);
      onTimeUpdate?.(video.currentTime);
    };
    const handlePlay = () => setIsPlaying(true);
    const handlePause = () => setIsPlaying(false);
    const handleEnded = () => setIsPlaying(false);
    const handleCanPlay = () => setIsReady(true);

    video.addEventListener('loadedmetadata', handleLoadedMetadata);
    video.addEventListener('timeupdate', handleTimeUpdate);
    video.addEventListener('play', handlePlay);
    video.addEventListener('pause', handlePause);
    video.addEventListener('ended', handleEnded);
    video.addEventListener('canplay', handleCanPlay);

    return () => {
      video.removeEventListener('loadedmetadata', handleLoadedMetadata);
      video.removeEventListener('timeupdate', handleTimeUpdate);
      video.removeEventListener('play', handlePlay);
      video.removeEventListener('pause', handlePause);
      video.removeEventListener('ended', handleEnded);
      video.removeEventListener('canplay', handleCanPlay);
    };
  }, [videoRef, onTimeUpdate]);

  return { isPlaying, currentTime, duration, isReady, volume, togglePlay, seekToTime, seek, setVolume };
}
