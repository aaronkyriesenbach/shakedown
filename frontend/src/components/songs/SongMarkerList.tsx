import { useState } from 'react';
import { Music, Plus, Edit2, Trash2, ChevronRight, Clock } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { formatDuration } from '@/lib/format';
import { cn } from '@/lib/utils';
import { getActiveTrack } from '@/lib/active-track';
import { useSongs, useDeleteSong, type Song } from '@/api/songs';
import { SongMarkerForm } from './SongMarkerForm';
import { toast } from 'sonner';

interface SongMarkerListProps {
  recordingId: string;
  onSeek?: (seconds: number) => void;
  currentTime?: number;
  /** When true, hides add/edit/delete controls. */
  readOnly?: boolean;
  /** When provided, uses these songs instead of fetching via useSongs. */
  songs?: Song[];
}

export function SongMarkerList({ recordingId, onSeek, currentTime = 0, readOnly, songs: songsProp }: SongMarkerListProps) {
  const { data: fetchedSongs, isLoading } = useSongs(recordingId, { enabled: !songsProp });
  const songs = songsProp ?? fetchedSongs;
  const deleteMutation = useDeleteSong(recordingId, '');
  
  const [isAdding, setIsAdding] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);

  const handleDelete = (e: React.MouseEvent, song: Song) => {
    e.stopPropagation();
    if (confirm(`Are you sure you want to delete the marker for "${song.title}"?`)) {
      deleteMutation.mutate(undefined, {
        onSuccess: () => toast.success('Marker deleted'),
        onError: () => toast.error('Failed to delete marker'),
      });
    }
  };

  const sortedSongs = [...(songs || [])].sort((a, b) => a.start_seconds - b.start_seconds);

  const sortedAsMarkers = sortedSongs.map((s) => ({
    id: s.id,
    title: s.title,
    startSeconds: s.start_seconds,
  }));
  const activeTrack = getActiveTrack(sortedAsMarkers, currentTime);

  if (isLoading) {
    return (
      <div className="space-y-4">
        <div className="flex justify-between items-center mb-4">
          <div className="h-8 w-32 bg-muted animate-pulse rounded" />
          <div className="h-8 w-24 bg-muted animate-pulse rounded" />
        </div>
        {[1, 2, 3].map((i) => (
          <div key={i} className="h-16 bg-muted animate-pulse rounded-md" />
        ))}
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between mb-4">
        <h3 className="text-lg font-medium flex items-center gap-2">
          <Music className="w-5 h-5 text-muted-foreground" />
          Song Markers
        </h3>
        {!readOnly && !isAdding && (
          <Button size="sm" onClick={() => setIsAdding(true)}>
            <Plus className="w-4 h-4 mr-2" />
            Add Marker
          </Button>
        )}
      </div>

      {!readOnly && isAdding && (
        <div className="mb-6">
          <SongMarkerForm 
            recordingId={recordingId} 
            currentTime={currentTime} 
            onClose={() => setIsAdding(false)} 
          />
        </div>
      )}

      {sortedSongs.length === 0 ? (
        !readOnly && !isAdding && (
          <div className="text-center py-12 border rounded-md border-dashed bg-muted/30">
            <Music className="w-10 h-10 mx-auto text-muted-foreground mb-3 opacity-50" />
            <p className="text-muted-foreground">No song markers yet.</p>
            <p className="text-sm text-muted-foreground mb-4">Add one to mark sections of this recording.</p>
            <Button variant="outline" size="sm" onClick={() => setIsAdding(true)}>
              <Plus className="w-4 h-4 mr-2" />
              Add First Marker
            </Button>
          </div>
        )
      ) : (
        <div className="space-y-2">
          {sortedSongs.map((song) => {
            const isActive = activeTrack?.id === song.id;
              
            if (!readOnly && editingId === song.id) {
              return (
                <div key={song.id} className="my-4">
                  <SongMarkerForm 
                    recordingId={recordingId} 
                    song={song}
                    currentTime={currentTime} 
                    onClose={() => setEditingId(null)} 
                  />
                </div>
              );
            }

            return (
              <button 
                type="button"
                key={song.id}
                onClick={() => onSeek?.(song.start_seconds)}
                className={cn(
                  "group flex flex-col w-full text-left p-3 rounded-md border transition-colors cursor-pointer relative overflow-hidden",
                  isActive 
                    ? "bg-violet-500/10 border-violet-500/30" 
                    : "bg-card hover:bg-muted/50 border-border"
                )}
              >
                {isActive && (
                  <div className="absolute left-0 top-0 bottom-0 w-1 bg-violet-500" />
                )}
                
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <div className={cn(
                      "flex items-center justify-center w-8 h-8 rounded-full",
                      isActive ? "bg-violet-500/20 text-violet-400" : "bg-muted text-muted-foreground"
                    )}>
                      {isActive ? <Music className="w-4 h-4" /> : <ChevronRight className="w-4 h-4 opacity-50 group-hover:opacity-100 transition-opacity" />}
                    </div>
                    
                    <div>
                      <div className="font-medium flex items-center gap-2">
                        {song.title}
                        {isActive && <span className="text-[10px] uppercase tracking-wider font-bold text-violet-400 bg-violet-500/20 px-1.5 py-0.5 rounded">Active</span>}
                      </div>
                      <div className="text-xs text-muted-foreground flex items-center gap-1 mt-0.5">
                        <Clock className="w-3 h-3" />
                        {formatDuration(song.start_seconds)}
                      </div>
                    </div>
                  </div>
                  
                  {!readOnly && (
                    <div className="flex items-center gap-1 opacity-0 pointer-events-none group-hover:opacity-100 group-hover:pointer-events-auto transition-opacity">
                      <Button 
                        variant="ghost" 
                        size="icon" 
                        className="h-8 w-8 text-muted-foreground hover:text-foreground"
                        onClick={(e) => {
                          e.stopPropagation();
                          setEditingId(song.id);
                        }}
                      >
                        <Edit2 className="w-4 h-4" />
                      </Button>
                      <Button 
                        variant="ghost" 
                        size="icon" 
                        className="h-8 w-8 text-muted-foreground hover:text-destructive"
                        onClick={(e) => handleDelete(e, song)}
                      >
                        <Trash2 className="w-4 h-4" />
                      </Button>
                    </div>
                  )}
                </div>
                
                {song.notes && (
                  <div className="mt-2 text-sm text-muted-foreground pl-11 line-clamp-2">
                    {song.notes}
                  </div>
                )}
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}
