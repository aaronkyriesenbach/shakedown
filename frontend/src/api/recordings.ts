import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { apiFetch } from '@/api/client';

export interface Recording {
  id: string;
  title: string;
  original_filename: string;
  file_ext: string;
  file_size_bytes: number;
  mime_type: string;
  storage_path: string;
  uploaded_by: string;
  recorded_at: string;
  recorded_at_source: string;
  duration_seconds?: number;
  bitrate?: number;
  sample_rate?: number;
  channels?: number;
  playback_ready: boolean;
  waveform_ready: boolean;
  processing_error?: string;
  created_at: string;
  updated_at: string;
  deleted_at?: string;
}

export interface CreateRecordingInput {
  title: string;
  original_filename: string;
  file_ext: string;
  file_size_bytes: number;
  mime_type: string;
  storage_path: string;
  uploaded_by: string;
  recorded_at: string;
  recorded_at_source: string;
}

export interface UpdateRecordingInput {
  title?: string;
  recorded_at?: string;
}

export interface ListFilter {
  search?: string;
  tag?: string;
  from?: string;
  to?: string;
  page?: number;
  limit?: number;
}

export interface ListResult<T> {
  data: T[];
  total: number;
  page: number;
  limit: number;
}

export const recordingKeys = {
  all: () => ['recordings'] as const,
  lists: () => ['recordings', 'list'] as const,
  list: (filter: ListFilter) => ['recordings', 'list', filter] as const,
  details: () => ['recordings', 'detail'] as const,
  detail: (id: string) => ['recordings', 'detail', id] as const,
};

export function useRecordings(filter: ListFilter = {}) {
  return useQuery({
    queryKey: recordingKeys.list(filter),
    queryFn: () => {
      const params = new URLSearchParams();
      if (filter.search) params.set('search', filter.search);
      if (filter.tag) params.set('tag', filter.tag);
      if (filter.from) params.set('from', filter.from);
      if (filter.to) params.set('to', filter.to);
      if (filter.page) params.set('page', filter.page.toString());
      if (filter.limit) params.set('limit', filter.limit.toString());
      
      const qs = params.toString();
      const path = qs ? `/api/recordings?${qs}` : '/api/recordings';
      return apiFetch<ListResult<Recording>>(path);
    },
  });
}

export function useRecording(id: string) {
  return useQuery({
    queryKey: recordingKeys.detail(id),
    queryFn: () => apiFetch<Recording>(`/api/recordings/${id}`),
    enabled: !!id,
  });
}

export function useUpdateRecording(id: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: UpdateRecordingInput) =>
      apiFetch<Recording>(`/api/recordings/${id}`, {
        method: 'PATCH',
        body: JSON.stringify(input),
      }),
    onSuccess: (data) => {
      queryClient.setQueryData(recordingKeys.detail(id), data);
      queryClient.invalidateQueries({ queryKey: recordingKeys.lists() });
    },
  });
}

export function useDeleteRecording(id: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () =>
      apiFetch<void>(`/api/recordings/${id}`, {
        method: 'DELETE',
      }),
    onSuccess: () => {
      queryClient.removeQueries({ queryKey: recordingKeys.detail(id) });
      queryClient.invalidateQueries({ queryKey: recordingKeys.lists() });
    },
  });
}

export function streamUrl(id: string): string {
  return `/api/recordings/${id}/stream`;
}

export function downloadUrl(id: string): string {
  return `/api/recordings/${id}/download`;
}

export function waveformUrl(id: string): string {
  return `/api/recordings/${id}/waveform`;
}

export function segmentUrl(id: string, start: number, duration: number): string {
  return `/api/recordings/${id}/segment?start=${start}&duration=${duration}`;
}
