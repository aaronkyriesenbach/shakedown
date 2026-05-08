import { Play, Pause } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { formatDuration } from '@/lib/format';
import { cn } from '@/lib/utils';

export interface AudioControlsProps {
  isPlaying: boolean;
  currentTime: number;
  duration: number;
  volume: number;
  isReady: boolean;
  onTogglePlay: () => void;
  onVolumeChange: (v: number) => void;
  onSeek: (fraction: number) => void;
  className?: string;
}

export function AudioControls({
  isPlaying,
  currentTime,
  duration,
  volume,
  isReady,
  onTogglePlay,
  onVolumeChange,
  className
}: AudioControlsProps) {
  return (
    <div className={cn('flex items-center gap-4', className)}>
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
      </div>
    </div>
  );
}
