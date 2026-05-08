import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { apiFetch } from '@/api/client';

export interface Tag {
  id: string;
  name: string;
  color: string;
  created_by: string;
  created_at: string;
}

export interface CreateTagInput {
  name: string;
  color: string;
}

export const tagKeys = {
  all: () => ['tags'] as const,
};

export function useTags() {
  return useQuery({
    queryKey: tagKeys.all(),
    queryFn: () => apiFetch<Tag[]>('/api/tags'),
  });
}

export function useCreateTag() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: CreateTagInput) =>
      apiFetch<Tag>('/api/tags', {
        method: 'POST',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: tagKeys.all() });
    },
  });
}

export function useAttachTag(recordingId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (tagId: string) =>
      apiFetch<void>(`/api/recordings/${recordingId}/tags`, {
        method: 'POST',
        body: JSON.stringify({ tag_id: tagId }),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['recordings'] });
    },
  });
}

export function useDetachTag(recordingId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (tagId: string) =>
      apiFetch<void>(`/api/recordings/${recordingId}/tags/${tagId}`, {
        method: 'DELETE',
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['recordings'] });
    },
  });
}
