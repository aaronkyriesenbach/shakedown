import { useState } from 'react';
import type { ChangeEvent } from 'react';
import { Cloud, CloudOff, RefreshCw, CheckCircle2, XCircle, ChevronDown, ChevronUp } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { useSyncStatus, useRunSync, useTestRemote, useSaveRemote } from '@/api/cloudsync';
import { ApiError } from '@/api/client';

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

        {status.enabled && (
          <Button 
            onClick={() => runSync.mutate()} 
            disabled={status.in_progress || runSync.isPending}
          >
            <RefreshCw className={`mr-2 h-4 w-4 ${status.in_progress || runSync.isPending ? 'animate-spin' : ''}`} />
            {status.in_progress ? 'Syncing...' : 'Sync Now'}
          </Button>
        )}
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
