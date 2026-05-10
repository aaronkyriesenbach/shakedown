import { useRef, useState, useCallback, useEffect, type RefObject } from 'react';

export interface UseVideoPlayerProps {
  videoRef: RefObject<HTMLVideoElement | null>;
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

export function useVideoPlayer({ videoRef, onTimeUpdate, onSeek }: UseVideoPlayerProps): UseVideoPlayerReturn {
  const [isPlaying, setIsPlaying] = useState(false);
  const [currentTime, setCurrentTime] = useState(0);
  const [duration, setDuration] = useState(0);
  const [isReady, setIsReady] = useState(false);
  const [volume, setVolumeState] = useState(1);

  const togglePlay = useCallback(() => {
    const video = videoRef.current;
    if (!video) return;
    if (video.paused) {
      void video.play();
    } else {
      video.pause();
    }
  }, [videoRef]);

  const seekToTime = useCallback((seconds: number) => {
    const video = videoRef.current;
    if (!video) return;
    const clamped = Math.max(0, Math.min(seconds, video.duration || 0));
    video.currentTime = clamped;
    onSeek?.(clamped);
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
