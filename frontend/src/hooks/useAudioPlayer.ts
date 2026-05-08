import { useEffect, useRef, useState, useCallback, type RefObject } from 'react';
import WaveSurfer from 'wavesurfer.js';

export interface UseAudioPlayerProps {
  containerRef: RefObject<HTMLElement | null>;
  audioUrl: string;
  peaksUrl?: string;
  duration?: number;
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
}

export function useAudioPlayer({
  containerRef,
  audioUrl,
  peaksUrl,
  duration: initialDuration,
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
      backend: 'WebAudio',
    });

    wavesurferRef.current = ws;

    ws.on('ready', () => {
      setIsReady(true);
      setDuration(ws.getDuration());
    });

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
      ws.destroy();
      wavesurferRef.current = null;
    };
  }, [containerRef, audioUrl, peaksLoaded, peaks, initialDuration, onTimeUpdate, onSeek]);

  const togglePlay = useCallback(() => {
    if (wavesurferRef.current) {
      wavesurferRef.current.playPause();
    }
  }, []);

  const seek = useCallback((fraction: number) => {
    if (wavesurferRef.current) {
      wavesurferRef.current.seekTo(Math.max(0, Math.min(1, fraction)));
    }
  }, []);
  
  const seekToTime = useCallback((seconds: number) => {
    if (wavesurferRef.current) {
      const totalDuration = wavesurferRef.current.getDuration() || duration;
      if (totalDuration > 0) {
        wavesurferRef.current.seekTo(Math.max(0, Math.min(1, seconds / totalDuration)));
      }
    }
  }, [duration]);

  const setVolume = useCallback((v: number) => {
    if (wavesurferRef.current) {
      wavesurferRef.current.setVolume(v);
      setVolumeState(v);
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
  };
}
