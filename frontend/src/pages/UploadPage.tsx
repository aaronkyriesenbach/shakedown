import { UploadForm } from '@/components/recordings/UploadForm';

export default function UploadPage() {
  return (
    <div className="container max-w-4xl space-y-6 md:space-y-8 animate-in fade-in slide-in-from-bottom-4 duration-500">
      <div>
        <h1 className="text-2xl md:text-3xl font-bold tracking-tight">Upload Recording</h1>
        <p className="text-muted-foreground mt-2 text-sm md:text-base">
          Upload audio or video files to your library. Supported formats include MP3, WAV, MP4, and more.
        </p>
      </div>

      <div className="bg-card text-card-foreground rounded-lg border shadow-sm p-6">
        <UploadForm />
      </div>
    </div>
  );
}
