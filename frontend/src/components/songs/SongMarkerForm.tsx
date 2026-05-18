import { useState } from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { useCreateSong, useUpdateSong, type Song } from '@/api/songs';
import { toast } from 'sonner';

interface SongMarkerFormProps {
  recordingId: string;
  song?: Song;
  currentTime?: number;
  onClose: () => void;
}

export function SongMarkerForm({ recordingId, song, currentTime, onClose }: SongMarkerFormProps) {
  const isEditing = !!song;
  
  const [title, setTitle] = useState(song?.title ?? '');
  const [startSeconds, setStartSeconds] = useState(
    song?.start_seconds?.toString() ?? (currentTime !== undefined ? currentTime.toFixed(0) : '0')
  );
  const [notes, setNotes] = useState(song?.notes ?? '');

  const createMutation = useCreateSong(recordingId);
  const updateMutation = useUpdateSong(recordingId, song?.id ?? '');

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    
    if (!title.trim()) {
      toast.error('Title is required');
      return;
    }
    
    const start = parseInt(startSeconds, 10);
    if (isNaN(start) || start < 0) {
      toast.error('Valid start time is required');
      return;
    }
    
    if (isEditing) {
      updateMutation.mutate(
        {
          title: title.trim(),
          start_seconds: start,
          notes: notes.trim() || null,
        },
        {
          onSuccess: () => {
            toast.success('Song marker updated');
            onClose();
          },
          onError: () => {
            toast.error('Failed to update song marker');
          },
        }
      );
    } else {
      createMutation.mutate(
        {
          title: title.trim(),
          start_seconds: start,
          notes: notes.trim() || undefined,
        },
        {
          onSuccess: () => {
            toast.success('Song marker added');
            onClose();
          },
          onError: () => {
            toast.error('Failed to add song marker');
          },
        }
      );
    }
  };

  const isPending = createMutation.isPending || updateMutation.isPending;

  return (
    <form onSubmit={handleSubmit} className="space-y-4 p-4 border rounded-md bg-muted/30">
      <div className="space-y-1.5">
        <Label htmlFor="title">Title *</Label>
        <Input 
          id="title" 
          value={title} 
          onChange={(e) => setTitle(e.target.value)} 
          placeholder="e.g. Intro, Verse 1, Solo"
          autoFocus
        />
      </div>
      
      <div className="space-y-1.5">
        <Label htmlFor="start">Start (seconds) *</Label>
        <div className="relative">
          <Input 
            id="start" 
            type="number" 
            step="1"
            min="0"
            value={startSeconds} 
            onChange={(e) => setStartSeconds(e.target.value)}
            className={currentTime !== undefined ? 'pr-20' : ''}
          />
          {currentTime !== undefined && (
            <button
              type="button"
              onClick={() => setStartSeconds(currentTime.toFixed(0))}
              className="absolute right-2 top-1/2 -translate-y-1/2 text-xs text-primary hover:underline"
            >
              Use current
            </button>
          )}
        </div>
      </div>
      
      <div className="space-y-1.5">
        <Label htmlFor="notes">Notes</Label>
        <textarea 
          id="notes" 
          value={notes} 
          onChange={(e) => setNotes(e.target.value)} 
          placeholder="Optional"
          className="flex min-h-[80px] w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
        />
      </div>
      
      <div className="flex items-center justify-end gap-2 pt-2">
        <Button type="button" variant="ghost" onClick={onClose} disabled={isPending}>
          Cancel
        </Button>
        <Button type="submit" disabled={isPending}>
          {isEditing ? 'Save Changes' : 'Add Marker'}
        </Button>
      </div>
    </form>
  );
}
