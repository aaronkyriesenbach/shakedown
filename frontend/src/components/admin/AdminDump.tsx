import { Download, Database } from 'lucide-react';
import { adminDumpUrl } from '@/api/admin';
import { Button } from '@/components/ui/button';

export function AdminDump() {
  return (
    <div className="flex items-start gap-4">
      <div className="rounded-full bg-primary/10 p-3 text-primary">
        <Database className="h-6 w-6" />
      </div>
      <div className="flex-1 space-y-1">
        <h3 className="text-lg font-medium leading-none">Complete Data Dump</h3>
        <p className="text-sm text-muted-foreground">
          ZIP archive containing all audio files, metadata, songs, and comments.
        </p>
      </div>
      <Button asChild>
        <a href={adminDumpUrl()} download>
          <Download className="mr-2 h-4 w-4" />
          Download
        </a>
      </Button>
    </div>
  );
}
