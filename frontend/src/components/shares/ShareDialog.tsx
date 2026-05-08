import { useState } from 'react';
import { Copy, Check, Loader2 } from 'lucide-react';
import { toast } from 'sonner';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Button } from '@/components/ui/button';
import { useCreateShare } from '@/api/shares';
import type { Recording } from '@/api/recordings';

interface ShareDialogProps {
  recording: Recording;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function ShareDialog({ recording, open, onOpenChange }: ShareDialogProps) {
  const [label, setLabel] = useState('');
  const [startSeconds, setStartSeconds] = useState('');
  const [endSeconds, setEndSeconds] = useState('');
  const [expiresAt, setExpiresAt] = useState('');
  const [shareUrl, setShareUrl] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  const createMutation = useCreateShare();

  const handleOpenChange = (newOpen: boolean) => {
    if (!newOpen) {
      setTimeout(() => {
        setLabel('');
        setStartSeconds('');
        setEndSeconds('');
        setExpiresAt('');
        setShareUrl(null);
        setCopied(false);
        createMutation.reset();
      }, 300);
    }
    onOpenChange(newOpen);
  };

  const handleCreate = () => {
    const input: Parameters<typeof createMutation.mutate>[0] = {
      recording_id: recording.id,
    };

    if (label.trim()) input.label = label.trim();
    if (startSeconds) {
      const s = parseFloat(startSeconds);
      if (!isNaN(s)) input.start_seconds = s;
    }
    if (endSeconds) {
      const e = parseFloat(endSeconds);
      if (!isNaN(e)) input.end_seconds = e;
    }
    if (expiresAt) {
      input.expires_at = new Date(expiresAt).toISOString();
    }

    createMutation.mutate(input, {
      onSuccess: (data) => {
        const url = `${window.location.origin}/s/${data.token}`;
        setShareUrl(url);
      },
      onError: (err) => {
        toast.error(`Failed to create share: ${err instanceof Error ? err.message : 'Unknown error'}`);
      },
    });
  };

  const handleCopy = async () => {
    if (!shareUrl) return;
    try {
      await navigator.clipboard.writeText(shareUrl);
      setCopied(true);
      toast.success('Copied to clipboard');
      setTimeout(() => setCopied(false), 2000);
    } catch (err) {
      toast.error('Failed to copy to clipboard');
    }
  };

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-[425px]">
        <DialogHeader>
          <DialogTitle>Create Share Link</DialogTitle>
          <DialogDescription>
            Share "{recording.title}" with anyone. Links are public.
          </DialogDescription>
        </DialogHeader>

        {shareUrl ? (
          <div className="py-4 space-y-4">
            <div className="p-4 bg-muted/50 rounded-lg space-y-2 border">
              <Label>Share Link</Label>
              <div className="flex gap-2">
                <Input readOnly value={shareUrl} className="font-mono text-sm bg-background" />
                <Button size="icon" variant="secondary" onClick={handleCopy}>
                  {copied ? <Check className="w-4 h-4 text-green-500" /> : <Copy className="w-4 h-4" />}
                </Button>
              </div>
            </div>
            <DialogFooter>
              <Button onClick={() => handleOpenChange(false)}>Close</Button>
            </DialogFooter>
          </div>
        ) : (
          <>
            <div className="grid gap-4 py-4">
              <div className="grid gap-2">
                <Label htmlFor="share-label">Label (optional)</Label>
                <Input
                  id="share-label"
                  placeholder="e.g. 'Bridge section feedback'"
                  value={label}
                  onChange={(e) => setLabel(e.target.value)}
                />
              </div>

              <div className="grid grid-cols-2 gap-4">
                <div className="grid gap-2">
                  <Label htmlFor="start-time">Start (seconds)</Label>
                  <Input
                    id="start-time"
                    type="number"
                    min="0"
                    placeholder="Full recording"
                    value={startSeconds}
                    onChange={(e) => setStartSeconds(e.target.value)}
                  />
                </div>
                <div className="grid gap-2">
                  <Label htmlFor="end-time">End (seconds)</Label>
                  <Input
                    id="end-time"
                    type="number"
                    min="0"
                    placeholder="Full recording"
                    value={endSeconds}
                    onChange={(e) => setEndSeconds(e.target.value)}
                  />
                </div>
              </div>

              <div className="grid gap-2">
                <Label htmlFor="expires">Expires At (optional)</Label>
                <Input
                  id="expires"
                  type="datetime-local"
                  value={expiresAt}
                  onChange={(e) => setExpiresAt(e.target.value)}
                />
              </div>
            </div>
            <DialogFooter>
              <Button variant="outline" onClick={() => handleOpenChange(false)}>
                Cancel
              </Button>
              <Button onClick={handleCreate} disabled={createMutation.isPending}>
                {createMutation.isPending && <Loader2 className="w-4 h-4 mr-2 animate-spin" />}
                Create Link
              </Button>
            </DialogFooter>
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}
