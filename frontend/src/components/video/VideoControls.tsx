import { Play, Pause, Maximize2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { formatDuration } from '@/lib/format';
import { cn } from '@/lib/utils';
import type { Song } from '@/api/songs';

export interface VideoControlsProps {
  isPlaying: boolean;
  currentTime: number;
  duration: number;
  volume: number;
  isReady: boolean;
  onTogglePlay: () => void;
  onVolumeChange: (v: number) => void;
  onSeek: (fraction: number) => void;
  onFullscreen?: () => void;
  songs?: Song[];
  onMarkerClick?: (startSeconds: number) => void;
  className?: string;
}

export function VideoControls({
  isPlaying, currentTime, duration, volume, isReady,
  onTogglePlay, onVolumeChange, onSeek, onFullscreen,
  songs, onMarkerClick, className
}: VideoControlsProps) {
  const progress = duration > 0 ? currentTime / duration : 0;

  return (
    <div className={cn('flex flex-col gap-2', className)}>
      {/* Seek bar with optional song marker overlays */}
      <div className="relative w-full h-2 bg-muted rounded-full cursor-pointer group"
        onClick={(e) => {
          const rect = e.currentTarget.getBoundingClientRect();
          const fraction = (e.clientX - rect.left) / rect.width;
          onSeek(Math.max(0, Math.min(1, fraction)));
        }}
      >
        {/* Progress fill */}
        <div
          className="absolute inset-y-0 left-0 bg-primary rounded-full transition-all"
          style={{ width: `${progress * 100}%` }}
        />
        {/* Song marker overlays */}
        {songs && duration > 0 && songs.map((song) => (
          <button
            key={song.id}
            type="button"
            className="absolute top-1/2 -translate-y-1/2 w-2 h-4 bg-amber-400 rounded-sm opacity-80 hover:opacity-100 hover:scale-110 transition-all cursor-pointer"
            style={{ left: `${(song.start_seconds / duration) * 100}%`, transform: 'translate(-50%, -50%)' }}
            onClick={(e) => {
              e.stopPropagation();
              onMarkerClick?.(song.start_seconds);
            }}
            title={song.title}
            aria-label={`Seek to ${song.title}`}
          />
        ))}
        {/* Scrubber thumb */}
        <div
          className="absolute top-1/2 -translate-y-1/2 w-3 h-3 bg-primary rounded-full shadow opacity-0 group-hover:opacity-100 transition-opacity"
          style={{ left: `${progress * 100}%`, transform: 'translate(-50%, -50%)' }}
        />
      </div>

      {/* Controls row */}
      <div className="flex items-center gap-4">
        <Button
          variant="ghost"
          size="icon"
          onClick={onTogglePlay}
          disabled={!isReady}
          className="h-10 w-10 shrink-0 rounded-full bg-primary/10 text-primary hover:bg-primary/20 hover:text-primary"
          aria-label={isPlaying ? 'Pause' : 'Play'}
        >
          {isPlaying ? <Pause className="h-5 w-5" /> : <Play className="h-5 w-5 ml-1" />}
        </Button>

        <div className="flex items-center gap-1.5 text-sm font-medium tabular-nums text-foreground/80 min-w-[100px]">
          <span>{formatDuration(currentTime)}</span>
          <span className="text-muted-foreground/50">/</span>
          <span className="text-muted-foreground">{formatDuration(duration)}</span>
        </div>

        <div className="flex items-center gap-2 ml-auto">
          <input
            type="range"
            min="0"
            max="1"
            step="0.01"
            value={volume}
            onChange={(e) => onVolumeChange(parseFloat(e.target.value))}
            disabled={!isReady}
            className="w-24 accent-primary"
            aria-label="Volume"
          />
          {onFullscreen && (
            <Button
              variant="ghost"
              size="icon"
              onClick={onFullscreen}
              className="h-8 w-8 text-muted-foreground hover:text-foreground"
              aria-label="Fullscreen"
            >
              <Maximize2 className="h-4 w-4" />
            </Button>
          )}
        </div>
      </div>
    </div>
  );
}
