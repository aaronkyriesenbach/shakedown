import { useState, useRef, useMemo } from 'react';
import { Link, useNavigate, useSearchParams } from 'react-router-dom';
import { ChevronLeft, Edit2, Trash2, Download, Share2, Tag as TagIcon } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { formatDuration, formatDate, formatFileSize } from '@/lib/format';
import { WaveformPlayer, type WaveformPlayerRef } from '@/components/audio/WaveformPlayer';
import { VideoPlayer, type VideoPlayerRef } from '@/components/video/VideoPlayer';
import { RecordingEditDialog } from './RecordingEditDialog';
import { SongMarkerList } from '@/components/songs/SongMarkerList';
import { ShareDialog } from '@/components/shares/ShareDialog';
import { ShareList } from '@/components/shares/ShareList';
import { downloadUrl, segmentUrl, useDeleteRecording, audioStreamUrl, type Recording } from '@/api/recordings';
import { useSongs } from '@/api/songs';
import { useComments } from '@/api/comments';
import { useRecordingShares } from '@/api/shares';
import { useMe } from '@/api/auth';
import { CommentThread } from '@/components/comments/CommentThread';
import { CommentForm } from '@/components/comments/CommentForm';
import type { Tag } from '@/api/tags';
import { toast } from 'sonner';

export type RecordingWithTags = Recording & { tags?: Tag[] };

interface RecordingDetailProps {
  recording: RecordingWithTags;
}

export function RecordingDetail({ recording }: RecordingDetailProps) {
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const deleteMutation = useDeleteRecording(recording.id);
  const waveformRef = useRef<WaveformPlayerRef>(null);
  const videoRef = useRef<VideoPlayerRef>(null);

  const validTabs = ['songs', 'comments', 'shares'] as const;
  type TabValue = typeof validTabs[number];
  const rawTab = searchParams.get('tab');
  const activeTab: TabValue = validTabs.includes(rawTab as TabValue) ? (rawTab as TabValue) : 'songs';

  const handleTabChange = (value: string) => {
    const next = new URLSearchParams(searchParams);
    if (value === 'songs') {
      next.delete('tab');
    } else {
      next.set('tab', value);
    }
    setSearchParams(next, { replace: true });
  };
  
  const [currentTime, setCurrentTime] = useState(0);
  const [showVideo, setShowVideo] = useState(true);
  const [transferTime, setTransferTime] = useState<number | undefined>(undefined);
  const [transferPlaying, setTransferPlaying] = useState(false);
  const [isEditDialogOpen, setIsEditDialogOpen] = useState(false);
  const [isDeleteDialogOpen, setIsDeleteDialogOpen] = useState(false);
  const [isShareDialogOpen, setIsShareDialogOpen] = useState(false);
  const [segmentStart, setSegmentStart] = useState('0');
  const [segmentDuration, setSegmentDuration] = useState('30');

  const { data: comments = [], isLoading: isLoadingComments } = useComments(recording.id);
  const { data: currentUser } = useMe();
  const { data: songs } = useSongs(recording.id);
  const { data: shares } = useRecordingShares(recording.id);

  const songMarkers = useMemo(
    () => songs?.map((s) => ({ title: s.title, startSeconds: s.start_seconds })),
    [songs],
  );

  const handleDelete = () => {
    deleteMutation.mutate(undefined, {
      onSuccess: () => {
        toast.success('Recording deleted');
        navigate('/');
      },
      onError: () => {
        toast.error('Failed to delete recording');
      }
    });
  };

  const handleSegmentDownload = () => {
    const start = parseFloat(segmentStart);
    const duration = parseFloat(segmentDuration);
    
    if (isNaN(start) || isNaN(duration) || duration <= 0 || start < 0) {
      toast.error('Invalid segment parameters');
      return;
    }
    
    window.open(segmentUrl(recording.id, start, duration), '_blank');
  };

  const handleVideoToggle = (checked: boolean | 'indeterminate') => {
    if (showVideo) {
      // Switching from video to audio-only
      const time = videoRef.current?.getCurrentTime() ?? 0;
      const playing = videoRef.current?.getIsPlaying() ?? false;
      videoRef.current?.stop();
      setTransferTime(time);
      setTransferPlaying(playing);
    } else {
      // Switching from audio-only to video
      const time = waveformRef.current?.getCurrentTime() ?? 0;
      const playing = waveformRef.current?.getIsPlaying() ?? false;
      waveformRef.current?.stop();
      setTransferTime(time);
      setTransferPlaying(playing);
    }
    setShowVideo(checked === true);
  };

  const handleSeek = (seconds: number) => {
    if (recording.media_type === 'video' && showVideo) {
      videoRef.current?.seekTo(seconds);
    } else {
      waveformRef.current?.seekTo(seconds);
    }
  };

  return (
    <div className="container max-w-5xl mx-auto py-6 space-y-6">
      <div className="flex items-center justify-between gap-3">
        <div className="flex items-center gap-4 min-w-0">
          <Button variant="ghost" size="icon" asChild className="shrink-0">
            <Link to="/">
              <ChevronLeft className="w-5 h-5" />
            </Link>
          </Button>
          <h1 className="text-2xl font-bold truncate">{recording.title}</h1>
        </div>
        <div className="flex items-center gap-2 shrink-0">
          <Button variant="outline" size="icon" className="sm:w-auto sm:px-3" onClick={() => setIsEditDialogOpen(true)}>
            <Edit2 className="w-4 h-4 sm:mr-2" />
            <span className="hidden sm:inline">Edit</span>
          </Button>
          <Button variant="destructive" size="icon" className="sm:w-auto sm:px-3" onClick={() => setIsDeleteDialogOpen(true)}>
            <Trash2 className="w-4 h-4 sm:mr-2" />
            <span className="hidden sm:inline">Delete</span>
          </Button>
        </div>
      </div>

      <div className="w-full">
        {recording.media_type === 'video' && showVideo ? (
          <VideoPlayer
            ref={videoRef}
            recording={recording}
            initialTime={transferTime}
            autoPlay={transferPlaying}
            onTimeUpdate={setCurrentTime}
            onSeek={handleSeek}
            showVideo={showVideo}
            onShowVideoChange={(show) => handleVideoToggle(show)}
            songs={songs ?? []}
            onMarkerClick={handleSeek}
          />
        ) : (
          <WaveformPlayer
            ref={waveformRef}
            recording={recording}
            audioUrlOverride={recording.media_type === 'video' ? audioStreamUrl(recording.id) : undefined}
            initialTime={transferTime}
            autoPlay={transferPlaying}
            onTimeUpdate={setCurrentTime}
            markers={songMarkers}
            songs={songs}
            showVideo={showVideo}
            onShowVideoChange={recording.media_type === 'video' ? (show) => handleVideoToggle(show) : undefined}
          />
        )}
      </div>

      <div className="mt-8 pt-4">
        <Tabs value={activeTab} onValueChange={handleTabChange} className="w-full">
          <TabsList className="grid w-full grid-cols-3 md:w-[500px]">
            <TabsTrigger value="songs">Songs</TabsTrigger>
            <TabsTrigger value="comments">Comments ({comments.length})</TabsTrigger>
            <TabsTrigger value="shares">Shares ({shares?.length ?? 0})</TabsTrigger>
          </TabsList>
          <TabsContent value="songs" className="p-0 border rounded-md mt-4 bg-card min-h-[200px]">
            <div className="p-6">
              <SongMarkerList recordingId={recording.id} onSeek={handleSeek} currentTime={currentTime} />
            </div>
          </TabsContent>
        <TabsContent value="comments" className="mt-4 space-y-6">
          <Card className="p-4 bg-neutral-900/30 border-neutral-800">
            <CommentForm recordingId={recording.id} currentTime={currentTime} />
          </Card>
          <Card className="p-6 bg-neutral-900/30 border-neutral-800">
            <CommentThread 
              comments={comments} 
              recordingId={recording.id}
              currentUserId={currentUser?.id}
              onSeek={handleSeek}
              isLoading={isLoadingComments}
            />
          </Card>
        </TabsContent>
        <TabsContent value="shares" className="mt-4">
          <Card className="p-6 bg-neutral-900/30 border-neutral-800">
            <ShareList recordingId={recording.id} />
          </Card>
        </TabsContent>
        </Tabs>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
        <div className="md:col-span-2 space-y-6">
          <Card className="p-6">
            <h3 className="font-semibold mb-4 text-lg">Metadata</h3>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-y-4 text-sm">
              <div>
                <span className="text-muted-foreground block mb-1">File Size</span>
                <span className="font-medium">{formatFileSize(recording.file_size_bytes)}</span>
              </div>
              <div>
                <span className="text-muted-foreground block mb-1">Duration</span>
                <span className="font-medium">{recording.duration_seconds ? formatDuration(recording.duration_seconds) : 'Unknown'}</span>
              </div>
              <div>
                <span className="text-muted-foreground block mb-1">Date Recorded</span>
                <span className="font-medium">{formatDate(recording.recorded_at)}</span>
              </div>
              {recording.media_type === 'video' && recording.video_width && recording.video_height && (
                <div>
                  <span className="text-muted-foreground block mb-1">Resolution</span>
                  <span className="font-medium">{recording.video_width} × {recording.video_height}</span>
                </div>
              )}
              {recording.media_type !== 'video' && (
                <div>
                  <span className="text-muted-foreground block mb-1">Channels</span>
                  <span className="font-medium">{recording.channels || 'Unknown'}</span>
                </div>
              )}
              {recording.media_type !== 'video' && (
                <div>
                  <span className="text-muted-foreground block mb-1">Sample Rate</span>
                  <span className="font-medium">{recording.sample_rate ? `${recording.sample_rate} Hz` : 'Unknown'}</span>
                </div>
              )}
              <div>
                <span className="text-muted-foreground block mb-1">Bitrate</span>
                <span className="font-medium">{recording.bitrate ? `${Math.round(recording.bitrate / 1000)} kbps` : 'Unknown'}</span>
              </div>
            </div>
          </Card>

          <Card className="p-6">
            <div className="flex items-center justify-between mb-4">
              <h3 className="font-semibold text-lg flex items-center gap-2">
                <TagIcon className="w-5 h-5 text-indigo-500" />
                Tags
              </h3>
              <Button variant="outline" size="sm" onClick={() => setIsEditDialogOpen(true)}>
                Manage Tags
              </Button>
            </div>
            {recording.tags && recording.tags.length > 0 ? (
              <div className="flex flex-wrap gap-2">
                {recording.tags.map(tag => (
                  <Badge 
                    key={tag.id} 
                    variant="secondary" 
                    style={{ 
                      backgroundColor: `${tag.color}20`, 
                      color: tag.color, 
                      borderColor: `${tag.color}40`,
                      borderWidth: '1px'
                    }}
                  >
                    {tag.name}
                  </Badge>
                ))}
              </div>
            ) : (
              <p className="text-sm text-muted-foreground">No tags attached.</p>
            )}
          </Card>
        </div>

        <div className="space-y-6">
          <Card className="p-6 flex flex-col gap-4">
            <h3 className="font-semibold text-lg">Actions</h3>
            
            <Button className="w-full justify-start" asChild>
              <a href={downloadUrl(recording.id)} download>
                <Download className="w-4 h-4 mr-2" />
                Download Original
              </a>
            </Button>

            <Button variant="outline" className="w-full justify-start" onClick={() => setIsShareDialogOpen(true)}>
              <Share2 className="w-4 h-4 mr-2" />
              Create Share Link
            </Button>

            <div className="pt-4 mt-2 border-t">
              <h4 className="text-sm font-medium mb-4">Segment Download</h4>
              <div className="grid grid-cols-2 gap-3 mb-4">
                <div className="space-y-1.5">
                  <Label htmlFor="seg-start" className="text-xs">Start (sec)</Label>
                  <div className="relative">
                    <Input 
                      id="seg-start" 
                      type="number" 
                      min="0" 
                      value={segmentStart} 
                      onChange={(e) => setSegmentStart(e.target.value)} 
                      className="h-8 text-sm pr-20"
                    />
                    <button
                      type="button"
                      onClick={() => setSegmentStart(Math.floor(currentTime).toString())}
                      className="absolute right-2 top-1/2 -translate-y-1/2 text-xs text-primary hover:underline"
                    >
                      Use current
                    </button>
                  </div>
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="seg-duration" className="text-xs">Duration (sec)</Label>
                  <Input 
                    id="seg-duration" 
                    type="number" 
                    min="1" 
                    value={segmentDuration} 
                    onChange={(e) => setSegmentDuration(e.target.value)} 
                    className="h-8 text-sm"
                  />
                </div>
              </div>
              <Button variant="secondary" className="w-full justify-start" onClick={handleSegmentDownload}>
                <Download className="w-4 h-4 mr-2" />
                Download Segment
              </Button>
            </div>
          </Card>
        </div>
      </div>

      <RecordingEditDialog 
        recording={recording}
        currentTagIds={recording.tags?.map(t => t.id) || []}
        open={isEditDialogOpen}
        onOpenChange={setIsEditDialogOpen}
      />

      <Dialog open={isDeleteDialogOpen} onOpenChange={setIsDeleteDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete Recording</DialogTitle>
            <DialogDescription>
              Are you sure you want to delete this recording? This action cannot be undone.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setIsDeleteDialogOpen(false)}>Cancel</Button>
            <Button variant="destructive" onClick={handleDelete} disabled={deleteMutation.isPending}>
              {deleteMutation.isPending ? 'Deleting...' : 'Delete'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
      
      <ShareDialog
        recording={recording}
        open={isShareDialogOpen}
        onOpenChange={setIsShareDialogOpen}
      />
    </div>
  );
}
