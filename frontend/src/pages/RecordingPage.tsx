import { useParams } from 'react-router-dom';
import { useRecording } from '@/api/recordings';
import { RecordingDetail } from '@/components/recordings/RecordingDetail';
import { Loader2 } from 'lucide-react';

export default function RecordingPage() {
  const { id } = useParams<{ id: string }>();
  
  const { data: recording, isLoading, isError } = useRecording(id || '');

  if (isLoading) {
    return (
      <div className="flex h-64 items-center justify-center">
        <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (isError || !recording) {
    return (
      <div className="flex h-64 items-center justify-center flex-col gap-4">
        <h2 className="text-xl font-semibold">Recording not found</h2>
        <p className="text-muted-foreground">The recording you are looking for does not exist or has been deleted.</p>
      </div>
    );
  }

  return <RecordingDetail recording={recording} />;
}
