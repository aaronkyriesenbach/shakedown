import { useState } from 'react';
import { Link } from 'react-router-dom';
import { Music, Video, Clock, HardDrive, AlertCircle, RefreshCw } from 'lucide-react';
import { Card } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';
import { formatDuration, formatDate, formatFileSize } from '@/lib/format';
import { type Recording, thumbnailUrl } from '@/api/recordings';
import type { Tag } from '@/api/tags';

export type RecordingWithTags = Recording & { tags?: Tag[] };

interface RecordingCardProps {
  recording: RecordingWithTags;
  className?: string;
}

export function RecordingCard({ recording, className }: RecordingCardProps) {
  const [thumbnailError, setThumbnailError] = useState(false);

  return (
    <Card className={cn("group hover:border-indigo-500/50 transition-colors overflow-hidden", className)}>
      <Link to={`/recordings/${recording.id}`} className="block p-3 sm:p-4">
        <div className="flex flex-col sm:flex-row sm:items-start sm:justify-between gap-2 sm:gap-4 mb-3">
          <div className="flex items-center gap-3">
            <div className="bg-muted w-10 h-10 rounded-md flex items-center justify-center flex-shrink-0 text-muted-foreground group-hover:text-indigo-400 transition-colors overflow-hidden relative">
              {recording.media_type === 'video' && recording.thumbnail_ready && !thumbnailError ? (
                <img
                  src={thumbnailUrl(recording.id)}
                  alt={recording.title}
                  className="w-full h-full object-cover"
                  loading="lazy"
                  onError={() => setThumbnailError(true)}
                />
              ) : recording.media_type === 'video' ? (
                <Video className="w-5 h-5" />
              ) : (
                <Music className="w-5 h-5" />
              )}
            </div>
            <div>
              <h3 className="font-semibold text-lg leading-tight line-clamp-2 sm:line-clamp-1 group-hover:text-indigo-400 transition-colors">
                {recording.title}
              </h3>
              <div className="flex flex-wrap items-center gap-x-3 gap-y-0.5 text-xs text-muted-foreground mt-1">
                <span className="flex items-center gap-1">
                  <Clock className="w-3 h-3" />
                  {recording.duration_seconds ? formatDuration(recording.duration_seconds) : "—"}
                </span>
                <span className="flex items-center gap-1">
                  <HardDrive className="w-3 h-3" />
                  {formatFileSize(recording.file_size_bytes)}
                </span>
                <span className="truncate">
                  {formatDate(recording.recorded_at)}
                </span>
              </div>
            </div>
          </div>
          
          <div className="flex flex-row sm:flex-col items-start sm:items-end gap-2 sm:flex-shrink-0">
            <Badge variant="outline" className="hidden sm:inline-flex text-[10px] px-1.5 py-0 h-4 border-muted-foreground/30 text-muted-foreground">
              {recording.media_type === 'video' ? (
                <><Video className="w-2.5 h-2.5 mr-1" />Video</>
              ) : (
                <><Music className="w-2.5 h-2.5 mr-1" />Audio</>
              )}
            </Badge>
            {recording.processing_error ? (
              <Badge variant="destructive" className="flex items-center gap-1">
                <AlertCircle className="w-3 h-3" />
                Error
              </Badge>
            ) : recording.processing_step !== 'complete' ? (
              <Badge variant="secondary" className="bg-amber-500/10 text-amber-500 hover:bg-amber-500/20 border-amber-500/20 flex items-center gap-1">
                <RefreshCw className="w-3 h-3 animate-spin" />
                {recording.processing_step === 'queued' && 'Queued'}
                {recording.processing_step === 'analyzing' && 'Analyzing'}
                {recording.processing_step === 'transcoding' && 'Transcoding'}
                {recording.processing_step === 'generating_waveform' && 'Generating waveform'}
                {recording.processing_step === 'extracting_thumbnail' && 'Extracting thumbnail'}
              </Badge>
            ) : null}
          </div>
        </div>

        {recording.tags && recording.tags.length > 0 && (
          <div className="flex flex-wrap gap-1.5 mt-3 pt-3 border-t">
            {recording.tags.map((tag) => (
              <Badge 
                key={tag.id} 
                variant="outline" 
                className="text-[10px] px-2 py-0 h-5"
                style={{ 
                  backgroundColor: tag.color,
                  borderColor: tag.color,
                  color: '#fff' 
                }}
              >
                {tag.name}
              </Badge>
            ))}
          </div>
        )}
      </Link>
    </Card>
  );
}
