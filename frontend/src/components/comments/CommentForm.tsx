import { useState } from 'react';
import { useCreateComment } from '@/api/comments';
import { Button } from '@/components/ui/button';
import { Loader2, Send, Clock } from 'lucide-react';
import { formatDuration } from '@/lib/format';
import { cn } from '@/lib/utils';
import { toast } from 'sonner';

export interface CommentFormProps {
  recordingId: string;
  parentId?: string;
  currentTime?: number;
  onClose?: () => void;
  placeholder?: string;
}

export function CommentForm({
  recordingId,
  parentId,
  currentTime = 0,
  onClose,
  placeholder = 'Add a comment...',
}: CommentFormProps) {
  const [content, setContent] = useState('');
  const [useTimestamp, setUseTimestamp] = useState(currentTime > 0 && !parentId);

  const { mutate: createComment, isPending } = useCreateComment(recordingId);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!content.trim()) return;

    createComment(
      {
        content: content.trim(),
        parent_id: parentId,
        timestamp_seconds: useTimestamp ? currentTime : undefined,
      },
      {
        onSuccess: () => {
          setContent('');
          setUseTimestamp(false);
          toast.success('Comment posted');
          onClose?.();
        },
        onError: () => {
          toast.error('Failed to post comment');
        },
      }
    );
  };

  return (
    <form onSubmit={handleSubmit} className="flex flex-col gap-3">
      <div className="relative">
        <textarea
          className="w-full min-h-[80px] p-3 text-sm rounded-md border border-neutral-800 bg-neutral-900/50 text-neutral-100 placeholder:text-neutral-500 focus:outline-none focus:ring-1 focus:ring-violet-500 resize-y"
          placeholder={placeholder}
          value={content}
          onChange={(e) => setContent(e.target.value)}
          disabled={isPending}
        />
        {!parentId && currentTime > 0 && (
          <div className="absolute bottom-3 left-3 flex items-center gap-2">
            <button
              type="button"
              onClick={() => setUseTimestamp(!useTimestamp)}
              className={cn(
                "flex items-center gap-1.5 px-2 py-1 rounded text-xs font-medium transition-colors",
                useTimestamp
                  ? "bg-violet-500/20 text-violet-300 hover:bg-violet-500/30"
                  : "bg-neutral-800 text-neutral-400 hover:bg-neutral-700 hover:text-neutral-300"
              )}
            >
              <Clock className="w-3.5 h-3.5" />
              <span>{formatDuration(currentTime)}</span>
            </button>
          </div>
        )}
      </div>

      <div className="flex items-center justify-end gap-2">
        {onClose && (
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={onClose}
            disabled={isPending}
          >
            Cancel
          </Button>
        )}
        <Button
          type="submit"
          size="sm"
          disabled={!content.trim() || isPending}
          className="bg-violet-600 hover:bg-violet-700 text-white"
        >
          {isPending ? (
            <Loader2 className="w-4 h-4 mr-2 animate-spin" />
          ) : (
            <Send className="w-4 h-4 mr-2" />
          )}
          Post
        </Button>
      </div>
    </form>
  );
}
