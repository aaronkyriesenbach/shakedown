import { useState } from 'react';
import { Copy, Check, Trash2, ExternalLink } from 'lucide-react';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Card } from '@/components/ui/card';
import { useRecordingShares, useDeleteShare, type Share } from '@/api/shares';
import { formatDate, formatRelativeTime } from '@/lib/format';

interface ShareListProps {
  recordingId: string;
}

function ShareRow({ share, recordingId }: { share: Share; recordingId: string }) {
  const [copied, setCopied] = useState(false);
  const deleteMutation = useDeleteShare(recordingId, share.id);
  
  const shareUrl = `${window.location.origin}/s/${share.token}`;
  
  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(shareUrl);
      setCopied(true);
      toast.success('Copied to clipboard');
      setTimeout(() => setCopied(false), 2000);
    } catch {
      toast.error('Failed to copy to clipboard');
    }
  };

  const handleDelete = () => {
    if (window.confirm('Are you sure you want to delete this share link?')) {
      deleteMutation.mutate(undefined, {
        onSuccess: () => toast.success('Share link deleted'),
        onError: () => toast.error('Failed to delete share link')
      });
    }
  };

  const isExpired = share.expires_at ? new Date(share.expires_at) < new Date() : false;
  
  const badgeLabel = share.start_seconds !== undefined && share.end_seconds !== undefined 
    ? `Snippet ${share.start_seconds}s–${share.end_seconds}s`
    : 'Full';

  const label = share.label ?? 'Untitled';

  return (
    <Card className="flex flex-col sm:flex-row sm:items-center justify-between p-3 gap-4 bg-transparent shadow-none border-border/50">
      <div className="flex-1 min-w-0 space-y-1">
        <div className="flex items-center gap-2">
          <span className="font-medium truncate text-sm">{label}</span>
          <Badge variant="secondary" className="text-[10px] h-4 px-1.5 shrink-0">
            {badgeLabel}
          </Badge>
          {share.expires_at ? (
            isExpired ? (
              <Badge variant="destructive" className="text-[10px] h-4 px-1.5 shrink-0">Expired</Badge>
            ) : null
          ) : null}
        </div>
        
        <div className="flex items-center gap-2 text-xs text-muted-foreground flex-wrap">
          <span className="flex items-center gap-1">
            Created {formatRelativeTime(share.created_at)}
          </span>
          <span>&bull;</span>
          <span className="flex items-center gap-1">
            {share.access_count} view{share.access_count !== 1 ? 's' : ''}
          </span>
          <span>&bull;</span>
          <span className="flex items-center gap-1">
            Expires:{' '}
            {share.expires_at ? (
              isExpired ? 'Expired' : formatDate(share.expires_at)
            ) : (
              'Never'
            )}
          </span>
        </div>
      </div>

      <div className="flex items-center gap-1 shrink-0">
        <Button variant="outline" size="sm" onClick={handleCopy} className="h-8 text-xs gap-1.5">
          {copied ? <Check className="w-3.5 h-3.5 text-green-500" /> : <Copy className="w-3.5 h-3.5" />}
          Copy
        </Button>
        <Button variant="ghost" size="icon" className="h-8 w-8" asChild>
          <a href={shareUrl} target="_blank" rel="noopener noreferrer" title="Open share link">
            <ExternalLink className="w-4 h-4 text-muted-foreground hover:text-primary" />
          </a>
        </Button>
        <Button 
          variant="ghost" 
          size="icon" 
          className="h-8 w-8 hover:text-destructive text-muted-foreground hover:bg-destructive/10" 
          onClick={handleDelete}
          disabled={deleteMutation.isPending}
          title="Delete share link"
        >
          <Trash2 className="w-4 h-4" />
        </Button>
      </div>
    </Card>
  );
}

export function ShareList({ recordingId }: ShareListProps) {
  const { data: shares, isLoading } = useRecordingShares(recordingId);

  if (isLoading) {
    return <p className="text-sm text-muted-foreground">Loading...</p>;
  }

  if (!shares || shares.length === 0) {
    return <p className="text-sm text-muted-foreground">No share links yet.</p>;
  }

  return (
    <div className="flex flex-col gap-3">
      {shares.map((share) => (
        <ShareRow key={share.id} share={share} recordingId={recordingId} />
      ))}
    </div>
  );
}
