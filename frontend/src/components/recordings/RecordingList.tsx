import { useState } from 'react';
import { Music, ChevronDown } from 'lucide-react';
import { useRecordings, type ListFilter } from '@/api/recordings';
import { RecordingCard, type RecordingWithTags } from './RecordingCard';
import { Card } from '@/components/ui/card';
import { cn } from '@/lib/utils';

interface DayGroup {
  dateKey: string;
  label: string;
  recordings: RecordingWithTags[];
}

interface MonthGroup {
  label: string;
  days: DayGroup[];
}

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

  const monthFmt = new Intl.DateTimeFormat('en-US', { month: 'long', year: 'numeric' });
  const dayFmt = new Intl.DateTimeFormat('en-US', { weekday: 'long', month: 'long', day: 'numeric' });

  const months: Record<string, MonthGroup> = {};

  for (const rec of recordings) {
    const d = new Date(rec.recorded_at);
    const monthKey = monthFmt.format(d);
    const dateKey = d.toISOString().slice(0, 10);
    const dayLabel = dayFmt.format(d);

    if (!months[monthKey]) months[monthKey] = { label: monthKey, days: [] };
    const month = months[monthKey];
    let day = month.days.find((g) => g.dateKey === dateKey);
    if (!day) {
      day = { dateKey, label: dayLabel, recordings: [] };
      month.days.push(day);
    }
    day.recordings.push(rec);
  }

  const sortedMonths = Object.values(months).sort((a, b) => {
    const timeA = new Date(a.days[0].recordings[0].recorded_at).getTime();
    const timeB = new Date(b.days[0].recordings[0].recorded_at).getTime();
    return timeB - timeA;
  });

  for (const month of sortedMonths) {
    month.days.sort((a, b) => b.dateKey.localeCompare(a.dateKey));
  }

  return (
    <div className="space-y-8">
      {sortedMonths.map((month) => (
        <div key={month.label} className="space-y-3">
          <h2 className="text-sm font-medium text-muted-foreground tracking-wider uppercase ml-1">
            {month.label}
          </h2>
          <div className="space-y-2">
            {month.days.map((day) => (
              <DayDropdown key={day.dateKey} day={day} />
            ))}
          </div>
        </div>
      ))}
    </div>
  );
}

interface DayDropdownProps {
  day: DayGroup;
}

function DayDropdown({ day }: DayDropdownProps) {
  const [open, setOpen] = useState(true);

  return (
    <div className="rounded-lg border bg-card/50">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="flex w-full items-center gap-2 px-4 py-2.5 text-left text-sm font-medium hover:bg-muted/50 transition-colors rounded-lg"
      >
        <ChevronDown
          className={cn(
            'w-4 h-4 text-muted-foreground transition-transform duration-200',
            !open && '-rotate-90',
          )}
        />
        <span className="text-base font-semibold">{day.label}</span>
        <span className="text-xs text-muted-foreground ml-auto">
          {day.recordings.length} {day.recordings.length === 1 ? 'recording' : 'recordings'}
        </span>
      </button>
      {open && (
        <div className="grid gap-3 px-3 pb-3">
          {day.recordings.map((recording) => (
            <RecordingCard key={recording.id} recording={recording} />
          ))}
        </div>
      )}
    </div>
  );
}
