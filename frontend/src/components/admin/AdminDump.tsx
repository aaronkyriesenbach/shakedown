import { Download, Archive, Database } from 'lucide-react';
import { adminDumpUrl } from '@/api/admin';
import { Button } from '@/components/ui/button';

export function AdminDump() {
  const currentDate = new Date().toLocaleString();

  return (
    <div className="space-y-4">
      <div className="flex items-start gap-4">
        <div className="rounded-full bg-primary/10 p-3 text-primary">
          <Database className="h-6 w-6" />
        </div>
        <div className="space-y-1">
          <h3 className="text-lg font-medium leading-none">Complete Data Dump</h3>
          <p className="text-sm text-muted-foreground">
            ZIP archive containing all audio files, metadata, songs, and comments.
          </p>
        </div>
      </div>
      
      <div className="flex items-center gap-4 rounded-lg border p-4 bg-muted/40">
        <Archive className="h-5 w-5 text-muted-foreground" />
        <div className="flex-1 space-y-1">
          <p className="text-sm font-medium leading-none">Always Freshly Generated</p>
          <p className="text-xs text-muted-foreground">
            Last generated: {currentDate}
          </p>
        </div>
        <Button asChild>
          <a href={adminDumpUrl()} download>
            <Download className="mr-2 h-4 w-4" />
            Download Data Dump
          </a>
        </Button>
      </div>
    </div>
  );
}
