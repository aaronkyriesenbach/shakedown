import { useEffect, useRef, useState, useCallback, type RefObject } from 'react';
import WaveSurfer from 'wavesurfer.js';

export interface UseAudioPlayerProps {
  containerRef: RefObject<HTMLElement | null>;
  audioUrl: string;
  peaksUrl?: string;
  duration?: number;
  initialTime?: number;
  autoPlay?: boolean;
  onTimeUpdate?: (time: number) => void;
  onSeek?: (time: number) => void;
}

export interface UseAudioPlayerReturn {
  isPlaying: boolean;
  currentTime: number;
  duration: number;
  isReady: boolean;
  volume: number;
  seek: (fraction: number) => void;
  seekToTime: (seconds: number) => void;
  togglePlay: () => void;
  setVolume: (v: number) => void;
  stop: () => void;
}

export function useAudioPlayer({
  containerRef,
  audioUrl,
  peaksUrl,
  duration: initialDuration,
  initialTime,
  autoPlay,
  onTimeUpdate,
  onSeek,
}: UseAudioPlayerProps): UseAudioPlayerReturn {
  const wavesurferRef = useRef<WaveSurfer | null>(null);
  const [isPlaying, setIsPlaying] = useState(false);
  const [currentTime, setCurrentTime] = useState(0);
  const [duration, setDuration] = useState(initialDuration || 0);
  const [isReady, setIsReady] = useState(false);
  const [volume, setVolumeState] = useState(1);
  const [peaks, setPeaks] = useState<number[][] | undefined>(undefined);
  const [peaksLoaded, setPeaksLoaded] = useState(false);

  useEffect(() => {
    let isMounted = true;

    async function fetchPeaks() {
      if (!peaksUrl) {
        setPeaksLoaded(true);
        return;
      }
      
      try {
        const response = await fetch(peaksUrl);
        if (response.ok) {
          const data = await response.json();
          if (isMounted && data && Array.isArray(data.data)) {
            setPeaks([data.data]);
          }
        }
      } catch (err) {
        console.error('Failed to fetch peaks', err);
      } finally {
        if (isMounted) {
          setPeaksLoaded(true);
        }
      }
    }

    fetchPeaks();

    return () => {
      isMounted = false;
    };
  }, [peaksUrl]);

  useEffect(() => {
    if (!containerRef.current || !peaksLoaded) return;

    const ws = WaveSurfer.create({
      container: containerRef.current,
      waveColor: 'rgba(99,102,241,0.4)',
      progressColor: '#6366f1',
      cursorColor: '#ffffff',
      barWidth: 2,
      barGap: 1,
      barRadius: 2,
      height: 'auto',
      url: audioUrl,
      peaks: peaks,
      duration: initialDuration,
      backend: 'MediaElement',
      dragToSeek: true,
    });

    wavesurferRef.current = ws;

    ws.on('ready', () => {
      setIsReady(true);
      setDuration(ws.getDuration());
    });

    // With preloaded peaks and WebAudio backend, 'ready' fires before the
    // audio buffer is fetched. Seek/play must wait for the media's 'canplay'.
    const needsInitialPlayback = (initialTime !== undefined && initialTime > 0) || autoPlay;
    const handleCanPlay = needsInitialPlayback ? () => {
      if (initialTime !== undefined && initialTime > 0) {
        ws.getMediaElement().currentTime = Math.min(initialTime, ws.getDuration());
      }
      if (autoPlay) {
        ws.play();
      }
    } : null;

    if (handleCanPlay) {
      ws.getMediaElement().addEventListener('canplay', handleCanPlay, { once: true });
    }

    ws.on('play', () => setIsPlaying(true));
    ws.on('pause', () => setIsPlaying(false));
    
    ws.on('timeupdate', (time: number) => {
      setCurrentTime(time);
      onTimeUpdate?.(time);
    });

    ws.on('finish', () => {
      setIsPlaying(false);
    });

    ws.on('seeking', (time: number) => {
      setCurrentTime(time);
      onSeek?.(time);
    });

    return () => {
      if (handleCanPlay) {
        ws.getMediaElement().removeEventListener('canplay', handleCanPlay);
      }
      ws.destroy();
      wavesurferRef.current = null;
      isWarmedUpRef.current = false;
    };
  }, [containerRef, audioUrl, peaksLoaded, peaks, initialDuration, initialTime, autoPlay, onTimeUpdate, onSeek]);

  // iOS Safari: the <audio> element ignores currentTime until activated by
  // play(). We call play()→pause() on the underlying element during the first
  // user-gesture seek to force activation without audible output. (#3896)
  const isWarmedUpRef = useRef(false);

  const togglePlay = useCallback(() => {
    const ws = wavesurferRef.current;
    if (!ws) return;
    ws.playPause();
  }, []);

  const seekToTime = useCallback((seconds: number) => {
    const ws = wavesurferRef.current;
    if (!ws) return;
    const totalDuration = ws.getDuration() || duration;
    if (totalDuration <= 0) return;

    if (!isWarmedUpRef.current && !ws.isPlaying()) {
      const mediaEl = ws.getMediaElement();
      const prevVolume = mediaEl.volume;
      mediaEl.volume = 0;
      void mediaEl.play().catch(() => {});
      mediaEl.pause();
      mediaEl.volume = prevVolume;
      isWarmedUpRef.current = true;
    }

    const clamped = Math.max(0, Math.min(seconds, totalDuration));
    ws.seekTo(clamped / totalDuration);
  }, [duration]);

  const seek = useCallback((fraction: number) => {
    const ws = wavesurferRef.current;
    if (!ws) return;
    const totalDuration = ws.getDuration() || duration;
    seekToTime(Math.max(0, Math.min(1, fraction)) * totalDuration);
  }, [duration, seekToTime]);

  const setVolume = useCallback((v: number) => {
    if (wavesurferRef.current) {
      wavesurferRef.current.setVolume(v);
      setVolumeState(v);
    }
  }, []);

  const stop = useCallback(() => {
    if (wavesurferRef.current && wavesurferRef.current.isPlaying()) {
      wavesurferRef.current.pause();
    }
  }, []);

  return {
    isPlaying,
    currentTime,
    duration,
    isReady,
    volume,
    seek,
    seekToTime,
    togglePlay,
    setVolume,
    stop,
  };
}
