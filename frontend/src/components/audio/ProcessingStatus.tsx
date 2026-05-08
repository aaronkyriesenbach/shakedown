import { Loader2, AlertCircle } from 'lucide-react';
import { cn } from '@/lib/utils';

export interface ProcessingStatusProps {
  playbackReady: boolean;
  waveformReady: boolean;
  processingError?: string;
  className?: string;
}

export function ProcessingStatus({ playbackReady, waveformReady, processingError, className }: ProcessingStatusProps) {
  if (processingError) {
    return (
      <div className={cn('flex items-center gap-2 text-sm text-destructive bg-destructive/10 px-3 py-2 rounded-md', className)}>
        <AlertCircle className="w-4 h-4" />
        <span>Processing failed: {processingError}</span>
      </div>
    );
  }

  if (!playbackReady) {
    return (
      <div className={cn('flex items-center gap-2 text-sm text-muted-foreground bg-muted/50 px-3 py-2 rounded-md', className)}>
        <Loader2 className="w-4 h-4 animate-spin" />
        <span>Processing audio...</span>
      </div>
    );
  }

  if (playbackReady && !waveformReady) {
    return (
      <div className={cn('flex items-center gap-2 text-xs text-muted-foreground/80', className)}>
        <Loader2 className="w-3 h-3 animate-spin" />
        <span>Generating waveform...</span>
      </div>
    );
  }

  return null;
}
