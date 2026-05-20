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
  const [shareMode, setShareMode] = useState<'full' | 'section'>('full');
  const [startSeconds, setStartSeconds] = useState('');
  const [endSeconds, setEndSeconds] = useState('');
  const [expiresAt, setExpiresAt] = useState('');
  const [shareUrl, setShareUrl] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  const createMutation = useCreateShare(recording.id);

  const handleOpenChange = (newOpen: boolean) => {
    if (!newOpen) {
      setTimeout(() => {
        setLabel('');
        setShareMode('full');
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
    if (shareMode === 'section') {
      const s = parseInt(startSeconds, 10);
      const e = parseInt(endSeconds, 10);
      if (isNaN(s) || isNaN(e) || s < 0 || e <= s) {
        toast.error('End time must be greater than start time');
        return;
      }
      input.start_seconds = s;
      input.end_seconds = e;
    }
    if (expiresAt) {
      const date = new Date(expiresAt + 'T23:59:59');
      input.expires_at = date.toISOString();
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
    } catch {
      toast.error('Failed to copy to clipboard');
    }
  };

  const sectionValid = shareMode === 'full' || (
    startSeconds !== '' && endSeconds !== '' &&
    !isNaN(parseInt(startSeconds, 10)) && !isNaN(parseInt(endSeconds, 10)) &&
    parseInt(endSeconds, 10) > parseInt(startSeconds, 10)
  );

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

              <div className="grid gap-2">
                <Label>Share</Label>
                <div className="flex w-full">
                  <button
                    type="button"
                    onClick={() => {
                      setShareMode('full');
                      setStartSeconds('');
                      setEndSeconds('');
                    }}
                    className={`flex-1 px-4 py-2 text-sm font-medium transition-colors rounded-l-md ${
                      shareMode === 'full'
                        ? 'bg-primary text-primary-foreground'
                        : 'bg-muted text-muted-foreground hover:bg-muted/80'
                    }`}
                  >
                    Full Recording
                  </button>
                  <button
                    type="button"
                    onClick={() => setShareMode('section')}
                    className={`flex-1 px-4 py-2 text-sm font-medium transition-colors rounded-r-md ${
                      shareMode === 'section'
                        ? 'bg-primary text-primary-foreground'
                        : 'bg-muted text-muted-foreground hover:bg-muted/80'
                    }`}
                  >
                    Section
                  </button>
                </div>
              </div>

              {shareMode === 'section' && (
                <div className="grid grid-cols-2 gap-4">
                  <div className="grid gap-2">
                    <Label htmlFor="start-time">Start (seconds)</Label>
                    <Input
                      id="start-time"
                      type="number"
                      step="1"
                      min="0"
                      required
                      placeholder="0"
                      value={startSeconds}
                      onChange={(e) => setStartSeconds(e.target.value)}
                    />
                  </div>
                  <div className="grid gap-2">
                    <Label htmlFor="end-time">End (seconds)</Label>
                    <Input
                      id="end-time"
                      type="number"
                      step="1"
                      min="0"
                      required
                      placeholder="End"
                      value={endSeconds}
                      onChange={(e) => setEndSeconds(e.target.value)}
                    />
                  </div>
                  {startSeconds !== '' && endSeconds !== '' && parseInt(endSeconds, 10) <= parseInt(startSeconds, 10) && (
                    <p className="col-span-2 text-sm text-destructive">End time must be greater than start time</p>
                  )}
                </div>
              )}

              <div className="grid gap-2">
                <Label htmlFor="expires">Expire after (optional)</Label>
                <Input
                  id="expires"
                  type="date"
                  value={expiresAt}
                  onChange={(e) => setExpiresAt(e.target.value)}
                />
              </div>
            </div>
            <DialogFooter>
              <Button variant="outline" onClick={() => handleOpenChange(false)}>
                Cancel
              </Button>
              <Button onClick={handleCreate} disabled={createMutation.isPending || !sectionValid}>
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
