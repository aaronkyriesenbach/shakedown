import { useMutation, useQuery } from '@tanstack/react-query';
import { apiFetch } from '@/api/client';
import type { Recording } from '@/api/recordings';

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

export type ShareWithRecording = Share & { recording?: Recording };

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
};

export function useCreateShare() {
  return useMutation({
    mutationFn: (input: CreateShareInput) =>
      apiFetch<Share>('/api/shares', {
        method: 'POST',
        body: JSON.stringify(input),
      }),
  });
}

export function useShare(token: string) {
  return useQuery({
    queryKey: shareKeys.detail(token),
    queryFn: () => apiFetch<ShareWithRecording>(`/api/s/${token}`),
    enabled: !!token,
  });
}

const BASE_URL = import.meta.env.VITE_API_URL ?? '';

export function shareStreamUrl(token: string) {
  return `${BASE_URL}/api/s/${token}/stream`;
}

export function shareDownloadUrl(token: string) {
  return `${BASE_URL}/api/s/${token}/download`;
}
