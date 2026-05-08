import { useState } from 'react';
import { useDeleteComment, useUpdateComment, type Comment } from '@/api/comments';
import { CommentForm } from './CommentForm';
import { Avatar, AvatarFallback } from '@/components/ui/avatar';
import { Button } from '@/components/ui/button';
import { formatDuration, formatRelativeTime } from '@/lib/format';
import { Reply, Edit2, Trash2, Loader2, MessageSquare } from 'lucide-react';
import { toast } from 'sonner';

export interface CommentThreadProps {
  comments: Comment[];
  recordingId: string;
  currentUserId?: string;
  onSeek?: (seconds: number) => void;
  isLoading?: boolean;
}

export function CommentThread({
  comments,
  recordingId,
  currentUserId,
  onSeek,
  isLoading,
}: CommentThreadProps) {
  if (isLoading) {
    return (
      <div className="space-y-6 animate-pulse">
        {[1, 2, 3].map((i) => (
          <div key={i} className="flex gap-4">
            <div className="w-8 h-8 rounded-full bg-neutral-800" />
            <div className="flex-1 space-y-2">
              <div className="flex items-center gap-2">
                <div className="h-4 w-24 bg-neutral-800 rounded" />
                <div className="h-3 w-16 bg-neutral-800 rounded" />
              </div>
              <div className="h-16 bg-neutral-800 rounded" />
            </div>
          </div>
        ))}
      </div>
    );
  }

  if (comments.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-12 text-neutral-500">
        <MessageSquare className="w-12 h-12 mb-4 opacity-20" />
        <p>No comments yet. Be the first to comment!</p>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {comments.map((comment) => (
        <CommentItem
          key={comment.id}
          comment={comment}
          recordingId={recordingId}
          currentUserId={currentUserId}
          onSeek={onSeek}
        />
      ))}
    </div>
  );
}

interface CommentItemProps {
  comment: Comment;
  recordingId: string;
  currentUserId?: string;
  onSeek?: (seconds: number) => void;
  isReply?: boolean;
}

function CommentItem({
  comment,
  recordingId,
  currentUserId,
  onSeek,
  isReply = false,
}: CommentItemProps) {
  const [isReplying, setIsReplying] = useState(false);
  const [isEditing, setIsEditing] = useState(false);
  const [editContent, setEditContent] = useState(comment.content);

  const { mutate: updateComment, isPending: isUpdating } = useUpdateComment(
    recordingId,
    comment.id
  );
  const { mutate: deleteComment, isPending: isDeleting } = useDeleteComment(
    recordingId,
    comment.id
  );

  const isAuthor = currentUserId === comment.author_id;
  const authorInitials = comment.author_id.substring(0, 2).toUpperCase();

  const handleUpdate = () => {
    if (!editContent.trim() || editContent === comment.content) {
      setIsEditing(false);
      return;
    }

    updateComment(
      { content: editContent.trim() },
      {
        onSuccess: () => {
          setIsEditing(false);
          toast.success('Comment updated');
        },
        onError: () => {
          toast.error('Failed to update comment');
        },
      }
    );
  };

  const handleDelete = () => {
    if (confirm('Are you sure you want to delete this comment?')) {
      deleteComment(undefined, {
        onSuccess: () => {
          toast.success('Comment deleted');
        },
        onError: () => {
          toast.error('Failed to delete comment');
        },
      });
    }
  };

  return (
    <div className="flex flex-col gap-4">
      <div className="flex gap-4 group">
        <Avatar className="w-8 h-8 border border-neutral-800">
          <AvatarFallback className="bg-neutral-900 text-xs text-neutral-400">
            {authorInitials}
          </AvatarFallback>
        </Avatar>

        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 mb-1">
            <span className="text-sm font-medium text-neutral-300">
              User {authorInitials}
            </span>
            <span className="text-xs text-neutral-500">
              {formatRelativeTime(comment.created_at)}
            </span>
            {comment.timestamp_seconds !== undefined && onSeek && (
              <button
                onClick={() => onSeek(comment.timestamp_seconds!)}
                className="px-2 py-0.5 text-[10px] font-medium bg-violet-500/10 text-violet-400 hover:bg-violet-500/20 rounded-full transition-colors"
              >
                at {formatDuration(comment.timestamp_seconds)}
              </button>
            )}
          </div>

          {isEditing ? (
            <div className="mt-2 space-y-3">
              <textarea
                className="w-full min-h-[80px] p-3 text-sm rounded-md border border-neutral-800 bg-neutral-900/50 text-neutral-100 placeholder:text-neutral-500 focus:outline-none focus:ring-1 focus:ring-violet-500 resize-y"
                value={editContent}
                onChange={(e) => setEditContent(e.target.value)}
                disabled={isUpdating}
                autoFocus
              />
              <div className="flex items-center gap-2">
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={() => {
                    setIsEditing(false);
                    setEditContent(comment.content);
                  }}
                  disabled={isUpdating}
                >
                  Cancel
                </Button>
                <Button
                  size="sm"
                  onClick={handleUpdate}
                  disabled={isUpdating || !editContent.trim()}
                >
                  {isUpdating && <Loader2 className="w-3 h-3 mr-2 animate-spin" />}
                  Save
                </Button>
              </div>
            </div>
          ) : (
            <div className="text-sm text-neutral-300 whitespace-pre-wrap break-words">
              {comment.content}
            </div>
          )}

          <div className="flex items-center gap-4 mt-2 opacity-0 group-hover:opacity-100 transition-opacity">
            {!isReply && (
              <button
                onClick={() => setIsReplying(!isReplying)}
                className="text-xs text-neutral-500 hover:text-neutral-300 flex items-center gap-1.5 transition-colors"
              >
                <Reply className="w-3.5 h-3.5" />
                Reply
              </button>
            )}
            
            {isAuthor && !isEditing && (
              <>
                <button
                  onClick={() => setIsEditing(true)}
                  className="text-xs text-neutral-500 hover:text-neutral-300 flex items-center gap-1.5 transition-colors"
                  disabled={isDeleting}
                >
                  <Edit2 className="w-3.5 h-3.5" />
                  Edit
                </button>
                <button
                  onClick={handleDelete}
                  className="text-xs text-neutral-500 hover:text-red-400 flex items-center gap-1.5 transition-colors"
                  disabled={isDeleting}
                >
                  {isDeleting ? (
                    <Loader2 className="w-3.5 h-3.5 animate-spin" />
                  ) : (
                    <Trash2 className="w-3.5 h-3.5" />
                  )}
                  Delete
                </button>
              </>
            )}
          </div>
        </div>
      </div>

      {isReplying && (
        <div className="pl-12">
          <CommentForm
            recordingId={recordingId}
            parentId={comment.id}
            onClose={() => setIsReplying(false)}
            placeholder="Write a reply..."
          />
        </div>
      )}

      {comment.replies && comment.replies.length > 0 && (
        <div className="pl-4 ml-4 border-l border-neutral-800/50 space-y-4">
          {comment.replies.map((reply) => (
            <CommentItem
              key={reply.id}
              comment={reply}
              recordingId={recordingId}
              currentUserId={currentUserId}
              onSeek={onSeek}
              isReply
            />
          ))}
        </div>
      )}
    </div>
  );
}
