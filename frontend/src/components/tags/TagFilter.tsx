import { useState, useEffect, useRef } from 'react';
import { useTags } from '@/api/tags';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import { Tag as TagIcon } from 'lucide-react';

interface TagFilterProps {
  onFilterChange: (tagIds: string[]) => void;
  className?: string;
}

export function TagFilter({ onFilterChange, className }: TagFilterProps) {
  const { data: tags, isLoading } = useTags();
  const [selectedTagIds, setSelectedTagIds] = useState<Set<string>>(new Set());
  const onFilterChangeRef = useRef(onFilterChange);
  onFilterChangeRef.current = onFilterChange;

  useEffect(() => {
    onFilterChangeRef.current(Array.from(selectedTagIds));
  }, [selectedTagIds]);

  // Single-select behavior: clicking an unselected tag selects it and clears others.
  // Clicking the already-selected tag deselects it.
  const toggleTag = (id: string) => {
    setSelectedTagIds((prev) => {
      const next = new Set<string>();
      if (prev.has(id)) {
        // Deselect: return empty set
        return next;
      }
      // Select only this id
      next.add(id);
      return next;
    });
  };

  const clearSelection = () => {
    setSelectedTagIds(new Set());
  };

  if (isLoading) {
    return (
      <div className={cn("flex flex-wrap gap-2 animate-pulse", className)}>
        <div className="h-8 w-16 bg-muted rounded-md" />
        <div className="h-8 w-24 bg-muted rounded-md" />
        <div className="h-8 w-20 bg-muted rounded-md" />
      </div>
    );
  }

  if (!tags || tags.length === 0) {
    return null;
  }

  return (
    <div className={cn("flex flex-wrap gap-2 items-center", className)}>
      <Button
        variant={selectedTagIds.size === 0 ? "default" : "outline"}
        size="sm"
        onClick={clearSelection}
        className="rounded-full h-8 px-4"
      >
        All
      </Button>
      {tags.map((tag) => {
        const isSelected = selectedTagIds.has(tag.id);
        return (
          <Button
            key={tag.id}
            variant={isSelected ? "default" : "outline"}
            size="sm"
            onClick={() => toggleTag(tag.id)}
            className="rounded-full h-8 px-4 flex items-center gap-1.5 transition-colors"
            style={isSelected ? { backgroundColor: tag.color, borderColor: tag.color, color: '#fff' } : { borderColor: tag.color, color: tag.color }}
          >
            <TagIcon className="w-3.5 h-3.5" />
            {tag.name}
          </Button>
        );
      })}
    </div>
  );
}
