import { useState, useEffect } from 'react';
import { useTags, useAttachTag, useDetachTag } from '@/api/tags';
import { useUpdateRecording, type Recording } from '@/api/recordings';
import { ApiError } from '@/api/client';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Badge } from '@/components/ui/badge';
import { toast } from 'sonner';
import { Loader2 } from 'lucide-react';
import { cn } from '@/lib/utils';

interface RecordingEditDialogProps {
  recording: Recording;
  currentTagIds?: string[];
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function RecordingEditDialog({
  recording,
  currentTagIds = [],
  open,
  onOpenChange,
}: RecordingEditDialogProps) {
  const { data: allTags, isLoading: isLoadingTags } = useTags();
  const updateRecording = useUpdateRecording(recording.id);
  const attachTag = useAttachTag(recording.id);
  const detachTag = useDetachTag(recording.id);

  const [title, setTitle] = useState(recording.title);
  const [recordedAt, setRecordedAt] = useState(() => {
    if (!recording.recorded_at) return '';
    const d = new Date(recording.recorded_at);
    return isNaN(d.getTime()) ? '' : d.toISOString().split('T')[0];
  });

  useEffect(() => {
    setTitle(recording.title);
    if (recording.recorded_at) {
      const d = new Date(recording.recorded_at);
      setRecordedAt(isNaN(d.getTime()) ? '' : d.toISOString().split('T')[0]);
    } else {
      setRecordedAt('');
    }
  }, [recording]);

  const handleSave = () => {
    updateRecording.mutate(
      {
        title: title.trim(),
        recorded_at: recordedAt ? new Date(recordedAt).toISOString() : undefined,
      },
      {
        onSuccess: () => {
          toast.success('Recording updated');
          onOpenChange(false);
        },
        onError: (err) => {
          const message = err instanceof ApiError
            ? err.userMessage
            : 'Failed to update recording';
          toast.error(message);
        },
      }
    );
  };

  const handleTagToggle = (tagId: string, isAttached: boolean) => {
    if (isAttached) {
      detachTag.mutate(tagId, {
        onError: () => toast.error('Failed to remove tag'),
      });
    } else {
      attachTag.mutate(tagId, {
        onError: () => toast.error('Failed to add tag'),
      });
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[425px]">
        <DialogHeader>
          <DialogTitle>Edit Recording</DialogTitle>
        </DialogHeader>

        <div className="grid gap-4 py-4">
          <div className="grid gap-2">
            <Label htmlFor="title">Title</Label>
            <Input
              id="title"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
            />
          </div>

          <div className="grid gap-2">
            <Label htmlFor="recordedAt">Date</Label>
            <Input
              id="recordedAt"
              type="date"
              value={recordedAt}
              onChange={(e) => setRecordedAt(e.target.value)}
            />
          </div>

          <div className="grid gap-2 mt-2">
            <Label>Tags</Label>
            <div className="flex flex-wrap gap-2">
              {isLoadingTags ? (
                <div className="flex items-center gap-2 text-sm text-muted-foreground">
                  <Loader2 className="h-4 w-4 animate-spin" />
                  Loading tags...
                </div>
              ) : allTags && allTags.length > 0 ? (
                allTags.map((tag) => {
                  const isAttached = currentTagIds.includes(tag.id);
                  const isPending =
                    (attachTag.isPending && attachTag.variables === tag.id) ||
                    (detachTag.isPending && detachTag.variables === tag.id);

                  return (
                    <Badge
                      key={tag.id}
                      variant="outline"
                      className={cn(
                        'cursor-pointer transition-colors',
                        isPending && 'opacity-50 pointer-events-none'
                      )}
                      style={
                        isAttached
                          ? {
                              backgroundColor: `${tag.color}20`,
                              color: tag.color,
                              borderColor: `${tag.color}40`,
                              borderWidth: '1px',
                            }
                          : {
                              borderColor: 'hsl(var(--border))',
                              color: 'hsl(var(--foreground))',
                              backgroundColor: 'transparent',
                            }
                      }
                      onClick={() => handleTagToggle(tag.id, isAttached)}
                    >
                      <div
                        className="mr-1.5 h-2 w-2 rounded-full"
                        style={{ backgroundColor: tag.color }}
                      />
                      {tag.name}
                      {isPending && <Loader2 className="ml-1 h-3 w-3 animate-spin" />}
                    </Badge>
                  );
                })
              ) : (
                <span className="text-sm text-muted-foreground">No tags available.</span>
              )}
            </div>
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button onClick={handleSave} disabled={updateRecording.isPending}>
            {updateRecording.isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
            Save Changes
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
