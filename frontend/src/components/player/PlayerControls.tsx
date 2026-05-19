import { Play, Pause, Maximize2, Video, VideoOff, SkipBack, SkipForward } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { formatDuration } from '@/lib/format';
import { cn } from '@/lib/utils';
import type { Song } from '@/api/songs';

export interface PlayerControlsProps {
  isPlaying: boolean;
  currentTime: number;
  duration: number;
  volume: number;
  isReady: boolean;
  onTogglePlay: () => void;
  onVolumeChange: (v: number) => void;
  /** When provided, renders a seek bar above the controls row. */
  onSeek?: (fraction: number) => void;
  /** When provided, renders a fullscreen button. */
  onFullscreen?: () => void;
  showVideo?: boolean;
  onShowVideoChange?: (show: boolean) => void;
  /** Song markers rendered on the seek bar (only used when onSeek is provided). */
  songs?: Song[];
  onMarkerClick?: (startSeconds: number) => void;
  onPreviousTrack?: () => void;
  onNextTrack?: () => void;
  className?: string;
}

export function PlayerControls({
  isPlaying, currentTime, duration, volume, isReady,
  onTogglePlay, onVolumeChange, onSeek, onFullscreen,
  showVideo, onShowVideoChange,
  songs, onMarkerClick, onPreviousTrack, onNextTrack, className,
}: PlayerControlsProps) {
  const progress = duration > 0 ? currentTime / duration : 0;

  return (
    <div className={cn('flex flex-col gap-2', className)}>
      {onSeek && (
        <div className="relative w-full h-2 bg-muted rounded-full cursor-pointer group"
          onClick={(e) => {
            const rect = e.currentTarget.getBoundingClientRect();
            const fraction = (e.clientX - rect.left) / rect.width;
            onSeek(Math.max(0, Math.min(1, fraction)));
          }}
        >
          <div
            className="absolute inset-y-0 left-0 bg-primary rounded-full transition-all"
            style={{ width: `${progress * 100}%` }}
          />
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
          <div
            className="absolute top-1/2 -translate-y-1/2 w-3 h-3 bg-primary rounded-full shadow opacity-0 group-hover:opacity-100 transition-opacity"
            style={{ left: `${progress * 100}%`, transform: 'translate(-50%, -50%)' }}
          />
        </div>
      )}

      <div className="flex items-center gap-2 sm:gap-4">
        {onPreviousTrack && (
          <Button
            variant="ghost"
            size="icon"
            onClick={onPreviousTrack}
            disabled={!isReady}
            className="h-8 w-8 shrink-0 text-muted-foreground hover:text-foreground"
            aria-label="Previous track"
          >
            <SkipBack className="h-4 w-4" />
          </Button>
        )}

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

        {onNextTrack && (
          <Button
            variant="ghost"
            size="icon"
            onClick={onNextTrack}
            disabled={!isReady}
            className="h-8 w-8 shrink-0 text-muted-foreground hover:text-foreground"
            aria-label="Next track"
          >
            <SkipForward className="h-4 w-4" />
          </Button>
        )}

        <div className="flex items-center gap-1.5 text-sm font-medium tabular-nums text-foreground/80 min-w-[100px]">
          <span>{formatDuration(currentTime)}</span>
          <span className="text-muted-foreground/50">/</span>
          <span className="text-muted-foreground">{formatDuration(duration)}</span>
        </div>

        <div className="flex items-center gap-2 ml-auto">
          {onShowVideoChange && (
            <Button
              variant="ghost"
              size="icon"
              onClick={() => onShowVideoChange(!showVideo)}
              className="h-8 w-8 text-muted-foreground hover:text-foreground"
              aria-label={showVideo ? 'Switch to audio only' : 'Show video'}
              title={showVideo ? 'Switch to audio only' : 'Show video'}
            >
              {showVideo ? <Video className="h-4 w-4" /> : <VideoOff className="h-4 w-4" />}
            </Button>
          )}
          <input
            type="range"
            min="0"
            max="1"
            step="0.01"
            value={volume}
            onChange={(e) => onVolumeChange(parseFloat(e.target.value))}
            disabled={!isReady}
            className="w-16 sm:w-24 accent-primary"
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
