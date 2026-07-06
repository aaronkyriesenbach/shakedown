import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { apiFetch } from '@/api/client';

export interface SyncStatus {
  enabled: boolean;
  reason: string;
  remote_configured?: boolean;
  remote_name?: string;
  counts?: {
    total: number;
    synced: number;
    pending: number;
    syncing: number;
    error: number;
  };
  in_progress?: boolean;
  last_reconcile_at?: string | null;
}

export const cloudSyncKeys = {
  all: ['cloudsync'] as const,
  status: () => [...cloudSyncKeys.all, 'status'] as const,
};

export function useSyncStatus() {
  return useQuery({
    queryKey: cloudSyncKeys.status(),
    queryFn: () => apiFetch<SyncStatus>('/api/admin/sync/status'),
    refetchInterval: 5000,
  });
}

export function useRunSync() {
  const queryClient = useQueryClient();
  
  return useMutation({
    mutationFn: () => apiFetch<void>('/api/admin/sync/run', { method: 'POST' }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: cloudSyncKeys.status() });
    },
  });
}

export function useTestRemote() {
  return useMutation({
    mutationFn: () => apiFetch<{ ok: boolean; reason?: string }>('/api/admin/sync/test', { method: 'POST' }),
  });
}

export function useSaveRemote() {
  const queryClient = useQueryClient();
  
  return useMutation({
    mutationFn: (configBlock: string) => apiFetch<void>('/api/admin/sync/remote', {
      method: 'POST',
      headers: {
        'Content-Type': 'text/plain',
      },
      body: configBlock,
    }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: cloudSyncKeys.status() });
    },
  });
}
