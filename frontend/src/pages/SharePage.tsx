import { useState, useRef, useMemo } from 'react';
import { useParams, Link } from 'react-router-dom';
import { Download, Loader2, Calendar, Clock, Music } from 'lucide-react';
import { useShare, shareStreamUrl, shareAudioStreamUrl, shareWaveformUrl, shareDownloadUrl } from '@/api/shares';
import { WaveformPlayer, type WaveformPlayerRef } from '@/components/audio/WaveformPlayer';
import { VideoPlayer, type VideoPlayerRef } from '@/components/video/VideoPlayer';
import { SongMarkerList } from '@/components/songs/SongMarkerList';
import { formatDuration, formatDate } from '@/lib/format';
import { Card } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { useTheme } from '@/hooks/useTheme';

export default function SharePage() {
  const { token } = useParams<{ token: string }>();
  const { data: share, isLoading, error } = useShare(token ?? '');
  const waveformRef = useRef<WaveformPlayerRef>(null);
  const videoRef = useRef<VideoPlayerRef>(null);
  const [showVideo, setShowVideo] = useState(true);
  const [transferTime, setTransferTime] = useState<number | undefined>(undefined);
  const [transferPlaying, setTransferPlaying] = useState(false);
  const [currentTime, setCurrentTime] = useState(0);
  useTheme();

  const songs = share?.songs;
  const songMarkers = useMemo(
    () => songs?.map((s) => ({ title: s.title, startSeconds: s.start_seconds })),
    [songs],
  );

  if (isLoading) {
    return (
      <div className="min-h-screen bg-background flex flex-col items-center justify-center p-4">
        <Loader2 className="w-8 h-8 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (error || !share) {
    return (
      <div className="min-h-screen bg-background flex flex-col items-center justify-center p-4">
        <div className="text-center space-y-4">
          <h1 className="text-2xl font-bold">Share Not Found</h1>
          <p className="text-muted-foreground">This share link is invalid or has expired.</p>
          <Button asChild>
            <Link to="/">Go to Shakedown</Link>
          </Button>
        </div>
      </div>
    );
  }

  const recording = share.recording;
  const hasSnippet = share.start_seconds !== undefined && share.end_seconds !== undefined;

  const handleSeek = (seconds: number) => {
    if (recording?.media_type === 'video' && showVideo) {
      videoRef.current?.seekTo(seconds);
    } else {
      waveformRef.current?.seekTo(seconds);
    }
  };

  const handleVideoToggle = (checked: boolean | 'indeterminate') => {
    if (showVideo) {
      const time = videoRef.current?.getCurrentTime() ?? 0;
      const playing = videoRef.current?.getIsPlaying() ?? false;
      videoRef.current?.stop();
      setTransferTime(time);
      setTransferPlaying(playing);
    } else {
      const time = waveformRef.current?.getCurrentTime() ?? 0;
      const playing = waveformRef.current?.getIsPlaying() ?? false;
      waveformRef.current?.stop();
      setTransferTime(time);
      setTransferPlaying(playing);
    }
    setShowVideo(checked === true);
  };

  return (
    <div className="min-h-screen bg-background flex flex-col">
      <header className="border-b bg-card/50 px-6 py-4">
        <div className="container max-w-4xl mx-auto flex items-center justify-between">
          <Link to="/" className="flex items-center gap-2 font-bold text-xl">
            <span className="text-[#6366F1]">Shakedown</span>
          </Link>
          {hasSnippet && (
            <Badge variant="secondary">
              Snippet: {formatDuration(share.start_seconds ?? 0)} – {formatDuration(share.end_seconds ?? 0)}
            </Badge>
          )}
        </div>
      </header>

      <main className="flex-1 container max-w-4xl mx-auto py-8 px-4 flex flex-col gap-6">
        <div className="flex flex-col gap-2">
          <h1 className="text-3xl font-bold break-words">
            {share.label || recording?.title || 'Shared Recording'}
          </h1>
          {recording && (
            <div className="flex flex-wrap gap-4 text-sm text-muted-foreground">
              <div className="flex items-center gap-1.5">
                <Calendar className="w-4 h-4" />
                {formatDate(recording.recorded_at)}
              </div>
              <div className="flex items-center gap-1.5">
                <Clock className="w-4 h-4" />
                {hasSnippet
                  ? formatDuration((share.end_seconds ?? 0) - (share.start_seconds ?? 0))
                  : recording.duration_seconds
                    ? formatDuration(recording.duration_seconds)
                    : 'Unknown'}
              </div>
              {recording.storage_path && (
                <div className="flex items-center gap-1.5">
                  <Music className="w-4 h-4" />
                  {recording.file_ext}
                </div>
              )}
            </div>
          )}
        </div>

        {recording ? (
          <Card className="p-6">
            {recording.media_type === 'video' && showVideo ? (
              <VideoPlayer
                ref={videoRef}
                recording={hasSnippet
                  ? { ...recording, duration_seconds: (share.end_seconds ?? 0) - (share.start_seconds ?? 0) }
                  : recording}
                streamUrlOverride={shareStreamUrl(share.token)}
                initialTime={transferTime}
                autoPlay={transferPlaying}
                onTimeUpdate={setCurrentTime}
                showVideo={showVideo}
                onShowVideoChange={(show) => handleVideoToggle(show)}
                songs={songs ?? []}
                onMarkerClick={handleSeek}
              />
            ) : (
              <WaveformPlayer
                ref={waveformRef}
                recording={hasSnippet
                  ? { ...recording, duration_seconds: (share.end_seconds ?? 0) - (share.start_seconds ?? 0) }
                  : recording}
                audioUrlOverride={recording.media_type === 'video'
                  ? shareAudioStreamUrl(share.token)
                  : shareStreamUrl(share.token)}
                peaksUrlOverride={shareWaveformUrl(share.token)}
                initialTime={transferTime}
                autoPlay={transferPlaying}
                onTimeUpdate={setCurrentTime}
                markers={songMarkers}
                songs={songs}
                showVideo={showVideo}
                onShowVideoChange={recording.media_type === 'video' ? (show) => handleVideoToggle(show) : undefined}
              />
            )}
          </Card>
        ) : (
          <Card className="p-6 flex flex-col items-center justify-center min-h-[200px] text-muted-foreground">
            <Music className="w-8 h-8 mb-4 opacity-50" />
            <p>Player requires recording metadata.</p>
          </Card>
        )}

        {songs && songs.length > 0 && recording && (
          <Card className="p-6">
            <SongMarkerList
              recordingId={recording.id}
              songs={songs}
              readOnly
              onSeek={handleSeek}
              currentTime={currentTime}
            />
          </Card>
        )}

        <div className="flex justify-end">
          <Button asChild variant="outline">
            <a href={shareDownloadUrl(share.token)} download>
              <Download className="w-4 h-4 mr-2" />
              Download
            </a>
          </Button>
        </div>
      </main>

      <footer className="border-t py-6 text-center text-sm text-muted-foreground">
        Powered by{' '}
        <Link to="/" className="text-[#6366F1] hover:underline font-medium">
          Shakedown
        </Link>
      </footer>
    </div>
  );
}
