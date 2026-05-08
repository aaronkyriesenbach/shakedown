import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiFetch } from '@/api/client';

export interface Comment {
  id: string;
  recording_id: string;
  song_id?: string;
  parent_id?: string;
  timestamp_seconds?: number;
  content: string;
  author_id: string;
  created_at: string;
  updated_at: string;
  deleted_at?: string;
  replies?: Comment[];
}

export interface CreateCommentInput {
  content: string;
  timestamp_seconds?: number;
  song_id?: string;
  parent_id?: string;
}

export interface UpdateCommentInput {
  content: string;
}

export const commentKeys = {
  all: ['comments'] as const,
  lists: () => [...commentKeys.all, 'list'] as const,
  list: (recordingId: string) => [...commentKeys.lists(), recordingId] as const,
  details: () => [...commentKeys.all, 'detail'] as const,
  detail: (id: string) => [...commentKeys.details(), id] as const,
};

export function useComments(recordingId: string) {
  return useQuery({
    queryKey: commentKeys.list(recordingId),
    queryFn: async (): Promise<Comment[]> => {
      return apiFetch<Comment[]>(`/api/recordings/${recordingId}/comments`);
    },
    enabled: !!recordingId,
  });
}

export function useCreateComment(recordingId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: CreateCommentInput): Promise<Comment> => {
      return apiFetch<Comment>(`/api/recordings/${recordingId}/comments`, {
        method: 'POST',
        body: JSON.stringify(input),
      });
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: commentKeys.list(recordingId) });
    },
  });
}

export function useUpdateComment(recordingId: string, commentId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: UpdateCommentInput): Promise<Comment> => {
      return apiFetch<Comment>(`/api/recordings/${recordingId}/comments/${commentId}`, {
        method: 'PATCH',
        body: JSON.stringify(input),
      });
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: commentKeys.list(recordingId) });
    },
  });
}

export function useDeleteComment(recordingId: string, commentId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (): Promise<void> => {
      await apiFetch(`/api/recordings/${recordingId}/comments/${commentId}`, {
        method: 'DELETE',
      });
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: commentKeys.list(recordingId) });
    },
  });
}
