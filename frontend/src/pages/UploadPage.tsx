import { UploadForm } from '@/components/recordings/UploadForm';

export default function UploadPage() {
  return (
    <div className="container mx-auto py-4 md:py-8 px-4 max-w-4xl">
      <div className="mb-6 md:mb-8">
        <h1 className="text-2xl md:text-3xl font-bold tracking-tight">Upload Recording</h1>
        <p className="text-muted-foreground mt-2 text-sm md:text-base">
          Upload audio files to your library. Supported formats include MP3, WAV, and more.
        </p>
      </div>

      <div className="bg-card text-card-foreground rounded-lg border shadow-sm p-6">
        <UploadForm />
      </div>
    </div>
  );
}
