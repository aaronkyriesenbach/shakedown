import { useState } from 'react';
import { useTags, useCreateTag } from '@/api/tags';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Badge } from '@/components/ui/badge';
import { toast } from 'sonner';
import { Loader2, Tag, Check } from 'lucide-react';

interface TagManagerProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

const PRESET_COLORS = [
  '#6366f1', // Indigo
  '#ef4444', // Red
  '#22c55e', // Green
  '#f59e0b', // Amber
  '#3b82f6', // Blue
  '#ec4899', // Pink
  '#8b5cf6', // Violet
  '#14b8a6', // Teal
];

export function TagManager({ open, onOpenChange }: TagManagerProps) {
  const { data: tags, isLoading: isLoadingTags } = useTags();
  const createTag = useCreateTag();

  const [name, setName] = useState('');
  const [selectedColor, setSelectedColor] = useState(PRESET_COLORS[0]);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim()) return;

    createTag.mutate(
      { name: name.trim(), color: selectedColor },
      {
        onSuccess: () => {
          toast.success('Tag created successfully');
          setName('');
        },
        onError: () => {
          toast.error('Failed to create tag');
        },
      }
    );
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[425px]">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Tag className="h-5 w-5" />
            Manage Tags
          </DialogTitle>
        </DialogHeader>

        <div className="mt-4 space-y-6">
          <div className="space-y-3">
            <h4 className="text-sm font-medium leading-none">Existing Tags</h4>
            <div className="flex flex-wrap gap-2">
              {isLoadingTags ? (
                <div className="flex items-center gap-2 text-sm text-muted-foreground">
                  <Loader2 className="h-4 w-4 animate-spin" />
                  Loading tags...
                </div>
              ) : tags && tags.length > 0 ? (
                tags.map((tag) => (
                  <Badge
                    key={tag.id}
                    variant="secondary"
                    className="flex items-center gap-1.5 px-2.5 py-0.5"
                    style={{
                      backgroundColor: `${tag.color}20`,
                      color: tag.color,
                      borderColor: `${tag.color}40`,
                      borderWidth: '1px'
                    }}
                  >
                    <div
                      className="h-2 w-2 rounded-full"
                      style={{ backgroundColor: tag.color }}
                    />
                    {tag.name}
                  </Badge>
                ))
              ) : (
                <p className="text-sm text-muted-foreground">No tags found.</p>
              )}
            </div>
          </div>

          <div className="space-y-4">
            <h4 className="text-sm font-medium leading-none">Create New Tag</h4>
            <form onSubmit={handleSubmit} className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="tagName">Name</Label>
                <Input
                  id="tagName"
                  placeholder="e.g. Needs Review"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  maxLength={50}
                  required
                />
              </div>
              
              <div className="space-y-2">
                <Label>Color</Label>
                <div className="flex gap-2">
                  {PRESET_COLORS.map((color) => (
                    <button
                      key={color}
                      type="button"
                      onClick={() => setSelectedColor(color)}
                      className="h-8 w-8 rounded-full flex items-center justify-center border-2 transition-all"
                      style={{
                        backgroundColor: color,
                        borderColor: selectedColor === color ? 'hsl(var(--foreground))' : 'transparent',
                      }}
                      aria-label={`Select color ${color}`}
                    >
                      {selectedColor === color && (
                        <Check className="h-4 w-4 text-white drop-shadow-sm" />
                      )}
                    </button>
                  ))}
                </div>
              </div>

              <Button
                type="submit"
                className="w-full"
                disabled={!name.trim() || createTag.isPending}
              >
                {createTag.isPending ? (
                  <>
                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                    Creating...
                  </>
                ) : (
                  'Create Tag'
                )}
              </Button>
            </form>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
