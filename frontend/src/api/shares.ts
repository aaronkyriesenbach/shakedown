import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { apiFetch } from '@/api/client';
import type { Recording } from '@/api/recordings';
import type { Song } from '@/api/songs';

export interface Share {
  id: string;
  token: string;
  recording_id: string;
  song_id?: string;
  start_seconds?: number;
  end_seconds?: number;
  label?: string;
  created_by: string;
  expires_at?: string;
  access_count: number;
  created_at: string;
}

export type ShareWithRecording = Share & { recording?: Recording; songs?: Song[] };

export interface CreateShareInput {
  recording_id: string;
  song_id?: string;
  start_seconds?: number;
  end_seconds?: number;
  label?: string;
  expires_at?: string;
}

export const shareKeys = {
  all: () => ['shares'] as const,
  detail: (token: string) => ['shares', 'detail', token] as const,
  byRecording: (recordingId: string) => ['recordings', recordingId, 'shares'] as const,
};

export function useCreateShare(recordingId?: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: CreateShareInput) =>
      apiFetch<Share>('/api/shares', {
        method: 'POST',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      if (recordingId) {
        queryClient.invalidateQueries({ queryKey: shareKeys.byRecording(recordingId) });
      }
    },
  });
}

export function useShare(token: string) {
  return useQuery({
    queryKey: shareKeys.detail(token),
    queryFn: () => apiFetch<ShareWithRecording>(`/api/s/${token}`),
    enabled: !!token,
  });
}

export function useRecordingShares(recordingId: string) {
  return useQuery({
    queryKey: shareKeys.byRecording(recordingId),
    queryFn: () => apiFetch<Share[]>(`/api/recordings/${recordingId}/shares`),
    enabled: !!recordingId,
  });
}

export function useDeleteShare(recordingId: string, shareId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () =>
      apiFetch<void>(`/api/recordings/${recordingId}/shares/${shareId}`, {
        method: 'DELETE',
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: shareKeys.byRecording(recordingId) });
    },
  });
}

const BASE_URL = import.meta.env.VITE_API_URL ?? '';

export function shareStreamUrl(token: string) {
  return `${BASE_URL}/api/s/${token}/stream`;
}

export function shareAudioStreamUrl(token: string) {
  return `${BASE_URL}/api/s/${token}/audio-stream`;
}

export function shareWaveformUrl(token: string) {
  return `${BASE_URL}/api/s/${token}/waveform`;
}

export function shareDownloadUrl(token: string) {
  return `${BASE_URL}/api/s/${token}/download`;
}

export function shareThumbnailUrl(token: string) {
  return `${BASE_URL}/api/s/${token}/thumbnail`;
}
