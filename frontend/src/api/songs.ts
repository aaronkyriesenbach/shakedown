import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { apiFetch } from '@/api/client';

export interface Song {
  id: string;
  recording_id: string;
  title: string;
  start_seconds: number;
  notes?: string;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface CreateSongInput {
  title: string;
  start_seconds: number;
  notes?: string;
}

export interface UpdateSongInput {
  title?: string;
  start_seconds?: number;
  notes?: string | null;
}

export const songKeys = {
  all: (recordingId: string) => ['recordings', recordingId, 'songs'] as const,
};

export function useSongs(recordingId: string, options?: { enabled?: boolean }) {
  return useQuery({
    queryKey: songKeys.all(recordingId),
    queryFn: () => apiFetch<Song[]>(`/api/recordings/${recordingId}/songs`),
    enabled: (options?.enabled ?? true) && !!recordingId,
  });
}

export function useCreateSong(recordingId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: CreateSongInput) =>
      apiFetch<Song>(`/api/recordings/${recordingId}/songs`, {
        method: 'POST',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: songKeys.all(recordingId) });
    },
  });
}

export function useUpdateSong(recordingId: string, songId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: UpdateSongInput) =>
      apiFetch<Song>(`/api/recordings/${recordingId}/songs/${songId}`, {
        method: 'PATCH',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: songKeys.all(recordingId) });
    },
  });
}

export function useDeleteSong(recordingId: string, songId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () =>
      apiFetch<void>(`/api/recordings/${recordingId}/songs/${songId}`, {
        method: 'DELETE',
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: songKeys.all(recordingId) });
    },
  });
}
