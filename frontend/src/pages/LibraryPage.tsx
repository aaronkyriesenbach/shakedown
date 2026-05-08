import { useState, useEffect, useCallback } from 'react';
import { Link } from 'react-router-dom';
import { Upload, Search, Calendar } from 'lucide-react';
import { useRecordings, type ListFilter } from '@/api/recordings';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { TagFilter } from '@/components/tags/TagFilter';
import { RecordingList } from '@/components/recordings/RecordingList';

export function LibraryPage() {
  const [searchInput, setSearchInput] = useState('');
  const [filter, setFilter] = useState<ListFilter>({});

  // Debounce search
  useEffect(() => {
    const handler = setTimeout(() => {
      setFilter((prev) => {
        const newSearch = searchInput || undefined;
        if (prev.search === newSearch) return prev;
        return { ...prev, search: newSearch };
      });
    }, 300);
    return () => clearTimeout(handler);
  }, [searchInput]);

  const handleTagsChange = useCallback((tagIds: string[]) => {
    setFilter((prev) => {
      const newTag = tagIds.length > 0 ? tagIds[0] : undefined;
      if (prev.tag === newTag) return prev;
      return { ...prev, tag: newTag };
    });
  }, []);

  const handleFromChange = useCallback((e: import('react').ChangeEvent<HTMLInputElement>) => {
    setFilter((prev) => {
      const newFrom = e.target.value || undefined;
      if (prev.from === newFrom) return prev;
      return { ...prev, from: newFrom };
    });
  }, []);

  const handleToChange = useCallback((e: import('react').ChangeEvent<HTMLInputElement>) => {
    setFilter((prev) => {
      const newTo = e.target.value || undefined;
      if (prev.to === newTo) return prev;
      return { ...prev, to: newTo };
    });
  }, []);

  const { data } = useRecordings(filter);
  const totalCount = data?.total || 0;

  return (
    <div className="container max-w-5xl py-8 space-y-8 animate-in fade-in duration-500">
      <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Library</h1>
          <p className="text-muted-foreground mt-1">
            {totalCount} {totalCount === 1 ? 'recording' : 'recordings'} available
          </p>
        </div>
        <Button asChild className="shrink-0 bg-indigo-600 hover:bg-indigo-700 text-white shadow-md shadow-indigo-500/20">
          <Link to="/upload">
            <Upload className="w-4 h-4 mr-2" />
            Upload Recording
          </Link>
        </Button>
      </div>

      <div className="bg-card border rounded-xl p-4 shadow-sm space-y-4">
        <div className="flex flex-col md:flex-row gap-4">
          <div className="relative flex-1">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
            <Input
              type="search"
              placeholder="Search recordings..."
              className="pl-9 bg-background/50 focus-visible:ring-indigo-500"
              value={searchInput}
              onChange={(e) => setSearchInput(e.target.value)}
            />
          </div>
          <div className="flex flex-col sm:flex-row sm:items-center gap-2">
            <div className="relative w-full sm:w-auto">
              <Calendar className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground pointer-events-none" />
              <Input
                type="date"
                aria-label="From date"
                className="pl-9 bg-background/50 w-full sm:w-[150px] focus-visible:ring-indigo-500"
                value={filter.from || ''}
                onChange={handleFromChange}
              />
            </div>
            <span className="text-muted-foreground text-sm text-center sm:text-left">to</span>
            <div className="relative w-full sm:w-auto">
              <Calendar className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground pointer-events-none" />
              <Input
                type="date"
                aria-label="To date"
                className="pl-9 bg-background/50 w-full sm:w-[150px] focus-visible:ring-indigo-500"
                value={filter.to || ''}
                onChange={handleToChange}
              />
            </div>
          </div>
        </div>

        <TagFilter onFilterChange={handleTagsChange} />
      </div>

      <RecordingList filter={filter} />
    </div>
  );
}

export default LibraryPage;
