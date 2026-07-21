import { useState } from 'react';
import type { ChangeEvent } from 'react';
import { Cloud, CloudOff, RefreshCw, CheckCircle2, XCircle, ChevronDown, ChevronUp, AlertTriangle } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { useSyncStatus, useRunSync, useTestRemote, useSaveRemote, useFailedSyncs, useRetryFailedSync } from '@/api/cloudsync';
import type { FailedSync, RetryStatus } from '@/api/cloudsync';
import { ApiError } from '@/api/client';

// retryStatusLabel renders the Retry Status glossary term (see CONTEXT.md)
// for a Failed Sync row: "Retrying now" for a row currently mid-retry,
// "Retrying" for one still eligible for automatic retry, and "Exhausted"
// once automatic retries have given up.
function retryStatusLabel(status: RetryStatus): string {
  switch (status) {
    case 'retrying_now':
      return 'Retrying now';
    case 'retrying':
      return 'Retrying';
    case 'exhausted':
      return 'Exhausted';
  }
}

function retryStatusBadgeClassName(status: RetryStatus): string {
  switch (status) {
    case 'retrying_now':
      return 'border-blue-500/20 bg-blue-500/10 text-blue-500';
    case 'retrying':
      return 'border-amber-500/20 bg-amber-500/10 text-amber-500';
    case 'exhausted':
      return 'border-destructive/20 bg-destructive/10 text-destructive';
  }
}

// FailedSyncRow renders a single Failed Sync: title, Error Class, the
// free-text error detail, a Retry Status indicator, and a Retry button.
// The Retry button is available regardless of Retry Status (Retrying or
// Exhausted alike) -- unlike Sync Now, which only re-claims still-Retrying
// rows. After a click, the row is expected to remain visible showing
// "Retrying now..." (retry_status becomes "retrying_now" on the next
// Failed Syncs refetch) until it clears (success) or reappears with an
// updated cause (failed again).
function FailedSyncRow({ row }: { row: FailedSync }) {
  const retry = useRetryFailedSync();
  const isRetryingNow = row.retry_status === 'retrying_now';

  return (
    <div className="flex flex-col gap-2 rounded-md border bg-background/60 p-3 sm:flex-row sm:items-center sm:justify-between">
      <div className="min-w-0 space-y-1">
        <p className="truncate text-sm font-medium">{row.title}</p>
        <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
          {row.error_class && (
            <span className="rounded bg-muted px-1.5 py-0.5 font-mono">{row.error_class}</span>
          )}
          {row.error && <span className="truncate">{row.error}</span>}
        </div>
        <p className="text-xs text-muted-foreground">
          Attempts: {row.attempts}
          {row.last_attempt_at && ` · Last attempt: ${new Date(row.last_attempt_at).toLocaleString()}`}
        </p>
        {retry.isError && (
          <p className="text-xs text-destructive">
            {retry.error instanceof ApiError ? retry.error.userMessage : retry.error.message}
          </p>
        )}
      </div>
      <div className="flex shrink-0 items-center gap-2">
        <Badge variant="outline" className={retryStatusBadgeClassName(row.retry_status)}>
          {retryStatusLabel(row.retry_status)}
        </Badge>
        <Button
          size="sm"
          variant="outline"
          onClick={() => retry.mutate(row.recording_id)}
          disabled={isRetryingNow || retry.isPending}
        >
          <RefreshCw className={`mr-1.5 h-3.5 w-3.5 ${isRetryingNow || retry.isPending ? 'animate-spin' : ''}`} />
          {isRetryingNow ? 'Retrying now…' : 'Retry'}
        </Button>
      </div>
    </div>
  );
}

// FailedSyncsSection is the expandable Failed Syncs list within
// CloudSyncCard: which recordings are failing, why (Error Class + error
// detail), and whether they're still Retrying or Exhausted. Fetched
// on-demand once expanded, separately from the aggregate /status endpoint.
function FailedSyncsSection({ errorCount }: { errorCount: number }) {
  const [expanded, setExpanded] = useState(false);
  const { data, isLoading, isError, error } = useFailedSyncs(expanded);

  if (errorCount <= 0) {
    return null;
  }

  return (
    <div className="space-y-2">
      <Button
        variant="outline"
        onClick={() => setExpanded((v) => !v)}
        className="w-full justify-between"
      >
        <span className="flex items-center gap-2">
          <AlertTriangle className="h-4 w-4 text-destructive" />
          Failed Syncs ({errorCount})
        </span>
        {expanded ? <ChevronUp className="ml-2 h-4 w-4" /> : <ChevronDown className="ml-2 h-4 w-4" />}
      </Button>

      {expanded && (
        <div className="space-y-2 rounded-md border bg-muted/30 p-3">
          {isLoading && (
            <p className="text-sm text-muted-foreground">Loading failed syncs...</p>
          )}
          {isError && (
            <p className="text-sm text-destructive">
              {error instanceof ApiError ? error.userMessage : error.message}
            </p>
          )}
          {!isLoading && !isError && data && data.failed_syncs.length === 0 && (
            <p className="text-sm text-muted-foreground">No failed syncs.</p>
          )}
          {!isLoading && !isError && data && data.failed_syncs.length > 0 && (
            <div className="space-y-2">
              {data.failed_syncs.map((row) => (
                <FailedSyncRow key={row.recording_id} row={row} />
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
}

export function CloudSyncCard() {
  const { data: status, isLoading } = useSyncStatus();
  const runSync = useRunSync();
  const testRemote = useTestRemote();
  const saveRemote = useSaveRemote();

  const [showConnect, setShowConnect] = useState(false);
  const [remoteConfig, setRemoteConfig] = useState('');

  if (isLoading || !status) {
    return (
      <div className="flex items-center justify-center p-6 text-muted-foreground animate-pulse">
        Loading sync status...
      </div>
    );
  }

  const handleSaveRemote = () => {
    saveRemote.mutate(remoteConfig, {
      onSuccess: () => {
        setRemoteConfig('');
        setShowConnect(false);
      }
    });
  };

  return (
    <div className="space-y-6">
      <div className="flex items-start gap-4">
        <div className={`rounded-full p-3 ${status.enabled ? 'bg-primary/10 text-primary' : 'bg-muted text-muted-foreground'}`}>
          {status.enabled ? <Cloud className="h-6 w-6" /> : <CloudOff className="h-6 w-6" />}
        </div>
        
        <div className="flex-1 space-y-1">
          <div className="flex items-center gap-2">
            <h3 className="text-lg font-medium leading-none">Cloud Sync</h3>
            {status.enabled ? (
              <span className="inline-flex items-center rounded-full bg-emerald-500/10 px-2 py-0.5 text-xs font-medium text-emerald-500 ring-1 ring-inset ring-emerald-500/20">
                Enabled
              </span>
            ) : (
              <span className="inline-flex items-center rounded-full bg-destructive/10 px-2 py-0.5 text-xs font-medium text-destructive ring-1 ring-inset ring-destructive/20">
                Disabled
              </span>
            )}
          </div>
          <p className="text-sm text-muted-foreground">
            {status.enabled && status.remote_name 
              ? `Syncing to remote: ${status.remote_name}` 
              : 'Backup recordings to external cloud storage'}
          </p>
        </div>

        <Button 
          onClick={() => runSync.mutate()} 
          disabled={!status.enabled || status.in_progress || runSync.isPending}
        >
          <RefreshCw className={`mr-2 h-4 w-4 ${status.in_progress || runSync.isPending ? 'animate-spin' : ''}`} />
          {status.in_progress ? 'Syncing...' : 'Sync Now'}
        </Button>
      </div>

      {!status.enabled && (
        <div className="rounded-lg border border-destructive/50 bg-destructive/5 p-4 flex items-center gap-3 text-destructive">
          <XCircle className="h-4 w-4 shrink-0" />
          <div className="text-sm">{status.reason}</div>
        </div>
      )}

      {status.enabled && status.counts && (
        <div className="grid grid-cols-5 gap-4 py-4 border-y">
          <div className="space-y-1 text-center">
            <p className="text-2xl font-bold">{status.counts.total}</p>
            <p className="text-xs text-muted-foreground uppercase tracking-wider">Total</p>
          </div>
          <div className="space-y-1 text-center">
            <p className="text-2xl font-bold text-emerald-500">{status.counts.synced}</p>
            <p className="text-xs text-muted-foreground uppercase tracking-wider">Synced</p>
          </div>
          <div className="space-y-1 text-center">
            <p className="text-2xl font-bold text-blue-500">{status.counts.syncing}</p>
            <p className="text-xs text-muted-foreground uppercase tracking-wider">Syncing</p>
          </div>
          <div className="space-y-1 text-center">
            <p className="text-2xl font-bold text-amber-500">{status.counts.pending}</p>
            <p className="text-xs text-muted-foreground uppercase tracking-wider">Pending</p>
          </div>
          <div className="space-y-1 text-center">
            <p className="text-2xl font-bold text-destructive">{status.counts.error}</p>
            <p className="text-xs text-muted-foreground uppercase tracking-wider">Error</p>
          </div>
        </div>
      )}

      {status.enabled && status.last_reconcile_at && (
        <p className="text-xs text-muted-foreground">
          Last reconcile: {new Date(status.last_reconcile_at).toLocaleString()}
        </p>
      )}

      {status.enabled && status.counts && (
        <FailedSyncsSection errorCount={status.counts.error} />
      )}

      {runSync.isError && (
        <div className="rounded-lg border border-destructive/50 bg-destructive/5 p-4 text-sm text-destructive">
          {runSync.error instanceof ApiError ? runSync.error.userMessage : runSync.error.message}
        </div>
      )}

      <div className="space-y-4">
        <div className="flex items-center gap-4">
          <Button 
            variant="outline" 
            onClick={() => setShowConnect(!showConnect)}
            className="w-full sm:w-auto justify-between"
          >
            Connect a remote
            {showConnect ? <ChevronUp className="ml-2 h-4 w-4" /> : <ChevronDown className="ml-2 h-4 w-4" />}
          </Button>

          {status.enabled && (
            <div className="flex items-center gap-2">
              <Button 
                variant="secondary" 
                onClick={() => testRemote.mutate()}
                disabled={testRemote.isPending}
              >
                Test Connection
              </Button>
              {testRemote.isSuccess && testRemote.data && (
                <div className="flex items-center gap-1 text-sm">
                  {testRemote.data.ok ? (
                    <span className="flex items-center text-emerald-500"><CheckCircle2 className="mr-1 h-4 w-4"/> OK</span>
                  ) : (
                    <span className="flex items-center text-destructive"><XCircle className="mr-1 h-4 w-4"/> Failed: {testRemote.data.reason}</span>
                  )}
                </div>
              )}
            </div>
          )}
        </div>

        {showConnect && (
          <div className="rounded-md bg-muted/50 p-4 space-y-4 border animate-in slide-in-from-top-2">
            <div className="space-y-2 text-sm text-muted-foreground">
              <p>Configure your cloud storage provider by pasting an `rclone` remote configuration block.</p>
              <p>For example, to connect Google Drive, run `rclone config` locally to create a remote, then paste the resulting section from your `rclone.conf` here:</p>
              <pre className="bg-background/80 p-2 rounded text-xs font-mono text-foreground border">
{`[my-drive]
type = drive
client_id = ...
client_secret = ...
scope = drive.file
token = {"access_token":"..."}`}
              </pre>
            </div>
            
            <textarea
              placeholder="Paste your rclone configuration block here..."
              className="flex min-h-[80px] w-full rounded-md border border-input bg-background px-3 py-2 text-sm font-mono ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50 h-40"
              value={remoteConfig}
              onChange={(e: ChangeEvent<HTMLTextAreaElement>) => setRemoteConfig(e.target.value)}
              disabled={saveRemote.isPending}
            />
            
            <div className="flex items-center justify-between">
              {saveRemote.isError && (
                <p className="text-sm text-destructive font-medium">
                  {saveRemote.error instanceof ApiError ? saveRemote.error.userMessage : saveRemote.error.message}
                </p>
              )}
              <div className="flex-1" />
              <Button 
                onClick={handleSaveRemote}
                disabled={!remoteConfig.trim() || saveRemote.isPending}
              >
                {saveRemote.isPending ? 'Saving...' : 'Save Configuration'}
              </Button>
            </div>
            {saveRemote.isSuccess && (
              <p className="text-sm text-emerald-500 text-right">Configuration saved successfully!</p>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
