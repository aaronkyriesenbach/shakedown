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

// RetryStatus mirrors the backend's derived Retry Status for a Failed Sync:
// "retrying" (still eligible for automatic retry), "retrying_now" (a retry
// attempt is in flight this instant, i.e. mid-retry), or "exhausted"
// (attempts >= max_attempts, automatic retries have given up). See
// CONTEXT.md for the glossary.
export type RetryStatus = 'retrying' | 'retrying_now' | 'exhausted';

export interface FailedSync {
  recording_id: string;
  title: string;
  error_class: string | null;
  error: string | null;
  attempts: number;
  last_attempt_at: string | null;
  next_attempt_at: string | null;
  retry_status: RetryStatus;
}

export const cloudSyncKeys = {
  all: ['cloudsync'] as const,
  status: () => [...cloudSyncKeys.all, 'status'] as const,
  failedSyncs: () => [...cloudSyncKeys.all, 'failed'] as const,
};

export function useSyncStatus() {
  return useQuery({
    queryKey: cloudSyncKeys.status(),
    queryFn: () => apiFetch<SyncStatus>('/api/admin/sync/status'),
    refetchInterval: 5000,
  });
}

// useFailedSyncs fetches the Failed Syncs list on demand (e.g. once the
// failures section is expanded), separately from the aggregate /status
// endpoint. Pass `enabled: false` to defer fetching until the admin
// actually wants to see the list. Polls while enabled so a row that was
// force-retried (via useRetryFailedSync) keeps reflecting its live outcome
// -- "Retrying now..." until it clears (success) or reappears with an
// updated cause (failed again) -- without requiring another manual action.
export function useFailedSyncs(enabled: boolean) {
  return useQuery({
    queryKey: cloudSyncKeys.failedSyncs(),
    queryFn: () => apiFetch<{ failed_syncs: FailedSync[] }>('/api/admin/sync/failed'),
    enabled,
    refetchInterval: enabled ? 5000 : false,
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

// useRetryFailedSync forces one immediate re-attempt of a single Failed
// Sync (see CONTEXT.md's "Retry" glossary entry), regardless of Retry
// Status -- unlike Sync Now, it bypasses the attempts < max_attempts gate.
// The request responds immediately; the row itself is expected to keep
// showing "Retrying now..." (retry_status="retrying_now") until the next
// Failed Syncs refetch reflects the outcome.
export function useRetryFailedSync() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (recordingId: string) =>
      apiFetch<void>(`/api/admin/sync/failed/${encodeURIComponent(recordingId)}/retry`, {
        method: 'POST',
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: cloudSyncKeys.failedSyncs() });
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
