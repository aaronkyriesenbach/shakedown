import { Music } from 'lucide-react';
import { useRecordings, type ListFilter } from '@/api/recordings';
import { RecordingCard, type RecordingWithTags } from './RecordingCard';
import { Card } from '@/components/ui/card';

interface RecordingListProps {
  filter: ListFilter;
}

export function RecordingList({ filter }: RecordingListProps) {
  const { data, isLoading, isError } = useRecordings(filter);

  if (isLoading) {
    return (
      <div className="space-y-6">
        {[1, 2, 3].map((i) => (
          <Card key={i} className="p-4 h-24 animate-pulse bg-card/50 flex items-center gap-4">
            <div className="w-10 h-10 bg-muted rounded-md flex-shrink-0" />
            <div className="space-y-2 flex-1">
              <div className="h-5 bg-muted rounded w-1/3" />
              <div className="h-3 bg-muted rounded w-1/4" />
            </div>
            <div className="w-20 h-6 bg-muted rounded-full flex-shrink-0" />
          </Card>
        ))}
      </div>
    );
  }

  if (isError) {
    return (
      <div className="p-8 text-center text-destructive border border-destructive/20 rounded-lg bg-destructive/10">
        Failed to load recordings.
      </div>
    );
  }

  const recordings = data?.data || [];
  const hasFilters = !!(filter.search || filter.tag || filter.from || filter.to);

  if (recordings.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-16 px-4 text-center border rounded-lg border-dashed">
        <div className="bg-muted w-16 h-16 rounded-full flex items-center justify-center text-muted-foreground mb-4">
          <Music className="w-8 h-8" />
        </div>
        <h3 className="text-lg font-semibold mb-1">No recordings found</h3>
        <p className="text-muted-foreground max-w-sm">
          {hasFilters
            ? "Try adjusting your search or filters to find what you're looking for."
            : "Get started by uploading your first audio recording."}
        </p>
      </div>
    );
  }

  const grouped = recordings.reduce((acc: Record<string, RecordingWithTags[]>, rec) => {
    const d = new Date(rec.recorded_at);
    const key = new Intl.DateTimeFormat('en-US', { month: 'long', year: 'numeric' }).format(d);
    if (!acc[key]) acc[key] = [];
    acc[key].push(rec);
    return acc;
  }, {});

  const groupKeys = Object.keys(grouped).sort((a, b) => {
    const timeA = new Date(grouped[a][0].recorded_at).getTime();
    const timeB = new Date(grouped[b][0].recorded_at).getTime();
    return timeB - timeA;
  });

  return (
    <div className="space-y-8">
      {groupKeys.map((group) => (
        <div key={group} className="space-y-3">
          <h2 className="text-sm font-medium text-muted-foreground tracking-wider uppercase ml-1">
            {group}
          </h2>
          <div className="grid gap-3">
            {grouped[group].map((recording) => (
              <RecordingCard key={recording.id} recording={recording} />
            ))}
          </div>
        </div>
      ))}
    </div>
  );
}
