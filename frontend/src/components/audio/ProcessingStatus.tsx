import { Loader2, AlertCircle, Check, Circle } from 'lucide-react';
import { cn } from '@/lib/utils';
import type { ProcessingStep } from '@/api/recordings';

export type { ProcessingStep };

export interface ProcessingStatusProps {
  processingStep: ProcessingStep;
  mediaType: 'audio' | 'video';
  processingError?: string;
  className?: string;
}

interface StepInfo {
  id: string;
  name: string;
  description: string;
}

const audioSteps: StepInfo[] = [
  { id: 'analyzing', name: 'Analyzing audio', description: 'Reading file metadata and audio properties' },
  { id: 'transcoding', name: 'Transcoding', description: 'Converting to optimized playback format' },
  { id: 'generating_waveform', name: 'Generating waveform', description: 'Creating visual waveform data' },
  { id: 'complete', name: 'Complete', description: 'Ready to play' },
];

const videoSteps: StepInfo[] = [
  { id: 'analyzing', name: 'Analyzing video', description: 'Reading file metadata and video properties' },
  { id: 'transcoding', name: 'Transcoding', description: 'Converting to optimized playback format' },
  { id: 'extracting_thumbnail', name: 'Extracting thumbnail', description: 'Extracting poster frame from video' },
  { id: 'extracting_audio', name: 'Extracting audio', description: 'Creating audio-only track for lightweight playback' },
  { id: 'generating_waveform', name: 'Generating waveform', description: 'Creating visual waveform data' },
  { id: 'complete', name: 'Complete', description: 'Ready to play' },
];

const audioStepOrder: readonly ProcessingStep[] = ['queued', 'analyzing', 'transcoding', 'generating_waveform', 'complete'];
const videoStepOrder: readonly ProcessingStep[] = ['queued', 'analyzing', 'transcoding', 'extracting_thumbnail', 'extracting_audio', 'generating_waveform', 'complete'];

export function ProcessingStatus({ processingStep, mediaType, processingError, className }: ProcessingStatusProps) {
  if (processingStep === 'complete' && !processingError) {
    return null;
  }

  const steps = mediaType === 'video' ? videoSteps : audioSteps;
  const stepOrder = mediaType === 'video' ? videoStepOrder : audioStepOrder;
  const currentIndex = stepOrder.indexOf(processingStep);

  return (
    <div className={cn('flex flex-col gap-3 p-4 bg-muted/20 rounded-lg', className)}>
      {processingStep === 'queued' && (
        <div className="text-sm font-medium text-muted-foreground mb-1">Waiting to start...</div>
      )}
      <div className="flex flex-col gap-4">
        {steps.map((step, index) => {
          const stepIndex = index + 1;
          const isPending = currentIndex < stepIndex;
          const isActive = currentIndex === stepIndex && !processingError;
          const isCompleted = currentIndex > stepIndex || (currentIndex === stepIndex && processingError);
          const hasError = currentIndex === stepIndex && processingError;

          return (
            <div 
              key={step.id} 
              className={cn(
                'flex items-start gap-3',
                isPending ? 'opacity-50' : 'opacity-100',
                isActive ? 'text-foreground' : 'text-muted-foreground'
              )}
            >
              <div className="mt-0.5 flex-shrink-0">
                {hasError ? (
                  <AlertCircle className="w-5 h-5 text-destructive" />
                ) : isActive ? (
                  <Loader2 className="w-5 h-5 animate-spin text-primary" />
                ) : isCompleted ? (
                  <Check className="w-5 h-5 text-green-500" />
                ) : (
                  <Circle className="w-5 h-5 text-muted-foreground" />
                )}
              </div>
              <div className="flex flex-col">
                <span className="text-sm font-medium">{step.name}</span>
                <span className="text-xs text-muted-foreground/80">{step.description}</span>
                {hasError && (
                  <div className="mt-1 text-xs text-destructive bg-destructive/10 px-2 py-1 rounded">
                    {processingError}
                  </div>
                )}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
