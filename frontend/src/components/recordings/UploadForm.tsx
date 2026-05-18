import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import Uppy, { type Meta, type Body } from '@uppy/core';
import type { UploadResult, UppyFile } from '@uppy/core';
import XHRUpload from '@uppy/xhr-upload';
import { toast } from 'sonner';
import { Upload, X, Music, Loader2, CheckCircle2, AlertCircle, RefreshCw, FolderOpen, ChevronDown } from 'lucide-react';

import { Badge } from '@/components/ui/badge';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { formatFileSize } from '@/lib/format';
import { extractRecordingDateInfo } from '@/lib/metadata';
import { cn } from '@/lib/utils';
import { apiFetch } from '@/api/client';
import type { Recording } from '@/api/recordings';

const SUPPORTED_EXTENSIONS = new Set(['.mp3', '.flac', '.wav', '.ogg', '.m4a', '.mp4', '.mov']);

function isSupportedFile(file: File): boolean {
  const mimeType = file.type.toLowerCase();
  if (mimeType.startsWith('audio/') || mimeType === 'video/mp4' || mimeType === 'video/quicktime') {
    return true;
  }
  const ext = ('.' + (file.name.split('.').pop() ?? '').toLowerCase());
  return SUPPORTED_EXTENSIONS.has(ext);
}

async function readAllDirectoryEntries(reader: FileSystemDirectoryReader): Promise<FileSystemEntry[]> {
  const entries: FileSystemEntry[] = [];
  let batch: FileSystemEntry[];
  do {
    batch = await new Promise<FileSystemEntry[]>((resolve, reject) =>
      reader.readEntries(resolve, reject),
    );
    entries.push(...batch);
  } while (batch.length > 0);
  return entries;
}

async function collectFilesFromEntry(entry: FileSystemEntry, files: File[]): Promise<void> {
  if (entry.isFile) {
    const file = await new Promise<File>((resolve, reject) =>
      (entry as FileSystemFileEntry).file(resolve, reject),
    );
    if (isSupportedFile(file)) {
      files.push(file);
    }
  } else if (entry.isDirectory) {
    const reader = (entry as FileSystemDirectoryEntry).createReader();
    const entries = await readAllDirectoryEntries(reader);
    await Promise.all(entries.map((child) => collectFilesFromEntry(child, files)));
  }
}

interface UploadMeta extends Meta {
  title?: string;
  recorded_at?: string;
}

interface RecordingBody extends Body {
  id: string;
  title: string;
  playback_ready: boolean;
}

type UploadResultType = {
  id?: string;
  title?: string;
  filename: string;
  success: boolean;
};

export function UploadForm() {
  const fileInputRef = useRef<HTMLInputElement>(null);
  const folderInputRef = useRef<HTMLInputElement>(null);
  const [files, setFiles] = useState<UppyFile<UploadMeta, RecordingBody>[]>([]);
  const [fileDates, setFileDates] = useState<Record<string, string>>({});
  const [fileTimestamps, setFileTimestamps] = useState<Record<string, string | null>>({});
  const [customTitles, setCustomTitles] = useState<Record<string, string>>({});
  const [analyzingFiles, setAnalyzingFiles] = useState<Set<string>>(new Set());
  const [isDragging, setIsDragging] = useState(false);
  const [isUploading, setIsUploading] = useState(false);
  const [uploadResults, setUploadResults] = useState<UploadResultType[] | null>(null);
  const [polledRecordings, setPolledRecordings] = useState<Record<string, Recording>>({});
  const analyzedFilesRef = useRef<Set<string>>(new Set());

  const [uppy] = useState(() => {
    const u = new Uppy<UploadMeta, RecordingBody>({
      restrictions: {
        allowedFileTypes: ['audio/*', 'video/mp4', 'video/quicktime'],
      },
    });

    u.use(XHRUpload, {
      endpoint: '/api/recordings',
      formData: true,
      fieldName: 'file',
      withCredentials: true,
    });

    return u;
  });

  const syncFiles = useCallback(() => {
    setFiles(uppy.getFiles());
  }, [uppy]);

  useEffect(() => {
    uppy.on('file-added', syncFiles);
    uppy.on('file-removed', syncFiles);
    uppy.on('upload-progress', syncFiles);

    const handleUploadStart = () => setIsUploading(true);
    const handleComplete = (result: UploadResult<UploadMeta, RecordingBody>) => {
      setIsUploading(false);
      const successful = result.successful ?? [];
      const failed = result.failed ?? [];

      const newResults: UploadResultType[] = [];

      successful.forEach((file: UppyFile<UploadMeta, RecordingBody>) => {
        toast.success(`Uploaded ${file.name} successfully`);
        const id = file.response?.body?.id;
        const title = file.response?.body?.title;
        newResults.push({ id, title, filename: file.name, success: true });
      });

      failed.forEach((file: UppyFile<UploadMeta, RecordingBody>) => {
        toast.error(`Failed to upload ${file.name}`);
        newResults.push({ filename: file.name, success: false });
      });

      setUploadResults(newResults);
    };

    uppy.on('upload', handleUploadStart);
    uppy.on('complete', handleComplete);

    return () => {
      uppy.off('file-added', syncFiles);
      uppy.off('file-removed', syncFiles);
      uppy.off('upload-progress', syncFiles);
      uppy.off('upload', handleUploadStart);
      uppy.off('complete', handleComplete);
    };
  }, [uppy, syncFiles]);


  useEffect(() => {
    const newFiles = files.filter(f => !analyzedFilesRef.current.has(f.id));
    if (newFiles.length === 0) return;

    for (const file of newFiles) {
      analyzedFilesRef.current.add(file.id);
    }

    const analyze = async () => {
      setAnalyzingFiles(prev => {
        const next = new Set(prev);
        for (const f of newFiles) next.add(f.id);
        return next;
      });

      const extractedDates: Record<string, string> = {};
      const extractedTimestamps: Record<string, string | null> = {};
      const today = new Date().toISOString().split('T')[0];

      await Promise.all(
        newFiles.map(async (file) => {
          try {
            const info = await extractRecordingDateInfo(file.data as File);
            extractedDates[file.id] = info?.date ?? today;
            extractedTimestamps[file.id] = info?.timestamp ?? null;
          } catch {
            extractedDates[file.id] = today;
            extractedTimestamps[file.id] = null;
          }
        }),
      );

      setFileDates(prev => {
        const merged = { ...prev };
        for (const [id, date] of Object.entries(extractedDates)) {
          if (merged[id] === undefined) {
            merged[id] = date;
          }
        }
        return merged;
      });

      setFileTimestamps(prev => {
        const merged = { ...prev };
        for (const [id, ts] of Object.entries(extractedTimestamps)) {
          if (merged[id] === undefined) {
            merged[id] = ts;
          }
        }
        return merged;
      });

      setAnalyzingFiles(prev => {
        const next = new Set(prev);
        for (const f of newFiles) next.delete(f.id);
        return next;
      });
    };

    analyze();
  }, [files]);

  useEffect(() => {
    for (const file of files) {
      const title = customTitles[file.id];
      const date = fileDates[file.id];
      if (title) uppy.setFileMeta(file.id, { title });
      if (date) uppy.setFileMeta(file.id, { recorded_at: date });
    }
  }, [files, customTitles, fileDates, uppy]);

  const polledRef = useRef(polledRecordings);
  polledRef.current = polledRecordings;

  useEffect(() => {
    if (!uploadResults) return;

    const successfulIds = uploadResults
      .filter((r) => r.success && r.id)
      .map((r) => r.id as string);

    if (successfulIds.length === 0) return;

    let cancelled = false;

    const poll = async () => {
      const pending = successfulIds.filter((id) => {
        const current = polledRef.current[id];
        return !current || current.processing_step !== 'complete';
      });
      if (pending.length === 0) return;

      for (const id of pending) {
        if (cancelled) return;
        try {
          const recording = await apiFetch<Recording>(`/api/recordings/${id}`);
          if (!cancelled) {
            setPolledRecordings((prev) => ({ ...prev, [id]: recording }));
          }
        } catch {
          // retry on next interval
        }
      }
    };

    poll();
    const interval = setInterval(poll, 2000);

    return () => {
      cancelled = true;
      clearInterval(interval);
    };
  }, [uploadResults]);

  const addFiles = useCallback((fileList: FileList | File[]) => {
    Array.from(fileList).forEach((file) => {
      try {
        uppy.addFile({
          name: file.name,
          type: file.type,
          data: file,
          source: 'Local',
        });
      } catch {
        toast.error(`Could not add ${file.name}`);
      }
    });
  }, [uppy]);

  const handleDrop = async (e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(false);

    const items = e.dataTransfer.items;
    if (!items?.length) return;

    const entries = Array.from(items)
      .map((item) => item.webkitGetAsEntry())
      .filter((entry): entry is FileSystemEntry => entry !== null);

    if (entries.length === 0) {
      // Fallback for browsers that don't support webkitGetAsEntry
      if (e.dataTransfer.files?.length > 0) {
        addFiles(e.dataTransfer.files);
      }
      return;
    }

    const files: File[] = [];
    await Promise.all(entries.map((entry) => collectFilesFromEntry(entry, files)));

    if (files.length === 0) {
      toast.info('No supported audio or video files found');
      return;
    }

    addFiles(files);
  };

  const handleDragOver = (e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(true);
  };

  const handleDragLeave = (e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(false);
  };

  const resetForm = () => {
    uppy.cancelAll();
    setUploadResults(null);
    setPolledRecordings({});
    setFileDates({});
    setFileTimestamps({});
    setCustomTitles({});
    setAnalyzingFiles(new Set());
    analyzedFilesRef.current.clear();
  };

  if (uploadResults) {
    return (
      <div className="space-y-6">
        <div className="flex flex-col items-center justify-center space-y-4 py-8 px-4 bg-muted/30 rounded-xl border border-dashed">
          <div className="w-12 h-12 bg-primary/10 text-primary rounded-full flex items-center justify-center">
            <CheckCircle2 className="w-6 h-6" />
          </div>
          <div className="text-center">
            <h3 className="text-lg font-medium">Upload Complete</h3>
            <p className="text-sm text-muted-foreground mt-1">
              {uploadResults.filter(r => r.success).length} of {uploadResults.length} files uploaded successfully.
            </p>
          </div>
          <Button onClick={resetForm} variant="outline" className="mt-2">
            <Upload className="w-4 h-4 mr-2" />
            Upload more
          </Button>
        </div>

        <div className="space-y-3">
          <h4 className="text-sm font-medium">Results</h4>
          <div className="space-y-2">
            {uploadResults.map((result, i) => {
              const recording = result.id ? polledRecordings[result.id] : null;
              const title = recording?.title || result.title || result.filename;
              
              return (
                <div key={`${result.filename}-${i}`} className="flex items-center justify-between p-3 rounded-lg border bg-card">
                  <div className="flex items-center gap-3 overflow-hidden">
                    <div className="bg-muted w-10 h-10 rounded-md flex items-center justify-center shrink-0">
                      <Music className="w-5 h-5 text-muted-foreground" />
                    </div>
                    <div className="min-w-0 flex-1">
                      {result.success && result.id ? (
                        <Link to={`/recordings/${result.id}`} className="font-medium text-sm truncate hover:underline block">
                          {title}
                        </Link>
                      ) : (
                        <p className="font-medium text-sm truncate">{title}</p>
                      )}
                      <p className="text-xs text-muted-foreground truncate">{result.filename}</p>
                    </div>
                  </div>
                  
                  <div className="ml-4 shrink-0">
                    {!result.success ? (
                      <Badge variant="destructive" className="flex items-center gap-1">
                        <AlertCircle className="w-3 h-3" />
                        Upload Failed
                      </Badge>
                    ) : recording?.processing_error ? (
                      <Badge variant="destructive" className="flex items-center gap-1">
                        <AlertCircle className="w-3 h-3" />
                        Error
                      </Badge>
                    ) : !recording || recording.processing_step !== 'complete' ? (
                      <Badge variant="secondary" className="bg-amber-500/10 text-amber-500 hover:bg-amber-500/20 border-amber-500/20 flex items-center gap-1">
                        <RefreshCw className="w-3 h-3 animate-spin" />
                         {recording?.processing_step === 'queued' ? 'Queued' :
                          recording?.processing_step === 'analyzing' ? 'Analyzing' :
                          recording?.processing_step === 'transcoding' ? 'Transcoding' :
                          recording?.processing_step === 'generating_waveform' ? 'Generating waveform' :
                          recording?.processing_step === 'extracting_thumbnail' ? 'Extracting thumbnail' : 'Processing'}
                       </Badge>
                    ) : (
                      <Badge className="bg-emerald-500/10 text-emerald-500 hover:bg-emerald-500/20 border-emerald-500/20 flex items-center gap-1">
                        <CheckCircle2 className="w-3 h-3" />
                        Ready
                      </Badge>
                    )}
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="space-y-4">
        <div
          className={`
            border-2 border-dashed rounded-xl p-8 text-center transition-colors
            ${isDragging ? 'border-primary bg-primary/5' : 'border-muted-foreground/25'}
            ${isUploading ? 'opacity-50 pointer-events-none' : ''}
          `}
          onDrop={handleDrop}
          onDragOver={handleDragOver}
          onDragLeave={handleDragLeave}
        >
          <input
            type="file"
            ref={fileInputRef}
            className="hidden"
            multiple
            accept="audio/*,video/mp4,video/quicktime"
            onChange={(e) => {
              if (e.target.files?.length) {
                addFiles(e.target.files);
              }
              e.target.value = '';
            }}
          />
          <input
            type="file"
            ref={folderInputRef}
            className="hidden"
            multiple
            webkitdirectory=""
            onChange={(e) => {
              if (e.target.files?.length) {
                const supported = Array.from(e.target.files).filter(isSupportedFile);
                const skipped = e.target.files.length - supported.length;
                if (skipped > 0) {
                  toast.info(`Skipped ${skipped} unsupported file${skipped === 1 ? '' : 's'}`);
                }
                if (supported.length > 0) {
                  addFiles(supported);
                } else {
                  toast.info('No supported audio or video files found in folder');
                }
              }
              e.target.value = '';
            }}
          />
          
          <div className="mx-auto w-12 h-12 mb-4 bg-muted rounded-full flex items-center justify-center">
            <Upload className="w-6 h-6 text-muted-foreground" />
          </div>
          
          <h3 className="text-lg font-semibold mb-1">
            Drag & drop files or folders
          </h3>
          <p className="text-sm text-muted-foreground mb-4">
            or browse for audio and video files from your computer
          </p>
          
          <div className="flex items-center gap-2 justify-center">
            <Button
              type="button"
              variant="secondary"
              size="sm"
              onClick={() => fileInputRef.current?.click()}
              disabled={isUploading}
            >
              <Upload className="w-4 h-4 mr-2" />
              Select Files
            </Button>
            <Button
              type="button"
              variant="secondary"
              size="sm"
              onClick={() => folderInputRef.current?.click()}
              disabled={isUploading}
            >
              <FolderOpen className="w-4 h-4 mr-2" />
              Select Folder
            </Button>
          </div>
        </div>
      </div>

      {files.length > 0 && (
        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <h4 className="text-sm font-medium">Selected Files ({files.length})</h4>
            {!isUploading && (
              <Button
                variant="ghost"
                size="sm"
                className="h-8 text-muted-foreground"
                onClick={() => {
                  uppy.cancelAll();
                  setFileDates({});
                  setFileTimestamps({});
                  setCustomTitles({});
                  setAnalyzingFiles(new Set());
                  analyzedFilesRef.current.clear();
                }}
              >
                Clear all
              </Button>
            )}
          </div>
          
          <GroupedFileList
            files={files}
            fileDates={fileDates}
            fileTimestamps={fileTimestamps}
            customTitles={customTitles}
            analyzingFiles={analyzingFiles}
            isUploading={isUploading}
            onTitleChange={(fileId, title) => {
              if (title.trim()) {
                setCustomTitles(prev => ({ ...prev, [fileId]: title }));
              } else {
                setCustomTitles(prev => {
                  const next = { ...prev };
                  delete next[fileId];
                  return next;
                });
              }
            }}
            onDateChange={(fileId, date) => {
              setFileDates(prev => ({ ...prev, [fileId]: date }));
            }}
            onRemoveFile={(fileId) => {
              uppy.removeFile(fileId);
              setCustomTitles(prev => {
                const next = { ...prev };
                delete next[fileId];
                return next;
              });
              setFileDates(prev => {
                const next = { ...prev };
                delete next[fileId];
                return next;
              });
              setFileTimestamps(prev => {
                const next = { ...prev };
                delete next[fileId];
                return next;
              });
              analyzedFilesRef.current.delete(fileId);
            }}
          />

          <Button
            className="w-full"
            size="lg"
            onClick={() => {
              const sortedIds = getSortedFileIds(files, fileDates, fileTimestamps);
              const currentIds = uppy.getFiles().map(f => f.id);
              const needsReorder = sortedIds.join(',') !== currentIds.join(',');
              
              if (needsReorder) {
                const fileDataMap = new Map(
                  uppy.getFiles().map(f => [f.id, f]),
                );
                uppy.cancelAll();
                for (const id of sortedIds) {
                  const f = fileDataMap.get(id);
                  if (!f?.data) continue;
                  const data = f.data as File | Blob;
                  uppy.addFile({
                    name: f.name,
                    type: f.type,
                    data,
                    source: f.source,
                    meta: f.meta,
                  });
                }
              }
              uppy.upload();
            }}
            disabled={isUploading || analyzingFiles.size > 0}
          >
            {isUploading ? (
              <>
                <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                Uploading...
              </>
            ) : analyzingFiles.size > 0 ? (
              <>
                <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                Analyzing files...
              </>
            ) : (
              <>
                <Upload className="w-4 h-4 mr-2" />
                Upload {files.length} {files.length === 1 ? 'File' : 'Files'}
              </>
            )}
          </Button>
        </div>
      )}
    </div>
  );
}

interface DayGroup {
  dateKey: string;
  label: string;
  fileIds: string[];
}

function getSortedFileIds(
  files: UppyFile<UploadMeta, RecordingBody>[],
  fileDates: Record<string, string>,
  fileTimestamps: Record<string, string | null>,
): string[] {
  const groups = groupFilesByDate(files, fileDates);
  const sorted: string[] = [];

  for (const group of groups) {
    const sortedWithinGroup = [...group.fileIds].sort((a, b) => {
      const tsA = fileTimestamps[a];
      const tsB = fileTimestamps[b];
      if (tsA && tsB) return tsA.localeCompare(tsB);
      if (tsA) return -1;
      if (tsB) return 1;
      const fileA = files.find(f => f.id === a);
      const fileB = files.find(f => f.id === b);
      return (fileA?.name ?? '').localeCompare(fileB?.name ?? '');
    });
    sorted.push(...sortedWithinGroup);
  }

  return sorted;
}

function groupFilesByDate(
  files: UppyFile<UploadMeta, RecordingBody>[],
  fileDates: Record<string, string>,
): DayGroup[] {
  const dayFmt = new Intl.DateTimeFormat('en-US', { weekday: 'long', month: 'long', day: 'numeric', year: 'numeric' });
  const groups: Record<string, DayGroup> = {};

  for (const file of files) {
    const dateKey = fileDates[file.id] || 'unknown';
    if (!groups[dateKey]) {
      const label = dateKey === 'unknown'
        ? 'Unknown Date'
        : dayFmt.format(new Date(dateKey + 'T12:00:00'));
      groups[dateKey] = { dateKey, label, fileIds: [] };
    }
    groups[dateKey].fileIds.push(file.id);
  }

  return Object.values(groups).sort((a, b) => b.dateKey.localeCompare(a.dateKey));
}

interface GroupedFileListProps {
  files: UppyFile<UploadMeta, RecordingBody>[];
  fileDates: Record<string, string>;
  fileTimestamps: Record<string, string | null>;
  customTitles: Record<string, string>;
  analyzingFiles: Set<string>;
  isUploading: boolean;
  onTitleChange: (fileId: string, title: string) => void;
  onDateChange: (fileId: string, date: string) => void;
  onRemoveFile: (fileId: string) => void;
}

function GroupedFileList({
  files,
  fileDates,
  fileTimestamps,
  customTitles,
  analyzingFiles,
  isUploading,
  onTitleChange,
  onDateChange,
  onRemoveFile,
}: GroupedFileListProps) {
  const groups = useMemo(
    () => groupFilesByDate(files, fileDates),
    [files, fileDates],
  );

  const sortedGroups = useMemo(() => {
    return groups.map(group => ({
      ...group,
      fileIds: [...group.fileIds].sort((a, b) => {
        const tsA = fileTimestamps[a];
        const tsB = fileTimestamps[b];
        if (tsA && tsB) return tsA.localeCompare(tsB);
        if (tsA) return -1;
        if (tsB) return 1;
        const fileA = files.find(f => f.id === a);
        const fileB = files.find(f => f.id === b);
        return (fileA?.name ?? '').localeCompare(fileB?.name ?? '');
      }),
    }));
  }, [groups, fileTimestamps, files]);

  const hasMultipleDates = groups.length > 1;
  const allAnalyzing = files.every(f => analyzingFiles.has(f.id));

  if (allAnalyzing || Object.keys(fileDates).length === 0) {
    return (
      <div className="space-y-2">
        {files.map((file) => (
          <FileRow
            key={file.id}
            file={file}
            fileDates={fileDates}
            customTitles={customTitles}
            analyzingFiles={analyzingFiles}
            isUploading={isUploading}
            onTitleChange={onTitleChange}
            onDateChange={onDateChange}
            onRemoveFile={onRemoveFile}
          />
        ))}
      </div>
    );
  }

  if (!hasMultipleDates) {
    const group = sortedGroups[0];
    return (
      <div className="space-y-2">
        {group.fileIds.map((fileId) => {
          const file = files.find(f => f.id === fileId);
          if (!file) return null;
          return (
            <FileRow
              key={file.id}
              file={file}
              fileDates={fileDates}
              customTitles={customTitles}
              analyzingFiles={analyzingFiles}
              isUploading={isUploading}
              onTitleChange={onTitleChange}
              onDateChange={onDateChange}
              onRemoveFile={onRemoveFile}
            />
          );
        })}
      </div>
    );
  }

  return (
    <div className="space-y-3">
      {sortedGroups.map((group) => (
        <DayDropdown
          key={group.dateKey}
          group={group}
          files={files}
          fileDates={fileDates}
          customTitles={customTitles}
          analyzingFiles={analyzingFiles}
          isUploading={isUploading}
          onTitleChange={onTitleChange}
          onDateChange={onDateChange}
          onRemoveFile={onRemoveFile}
        />
      ))}
    </div>
  );
}

interface DayDropdownProps {
  group: DayGroup;
  files: UppyFile<UploadMeta, RecordingBody>[];
  fileDates: Record<string, string>;
  customTitles: Record<string, string>;
  analyzingFiles: Set<string>;
  isUploading: boolean;
  onTitleChange: (fileId: string, title: string) => void;
  onDateChange: (fileId: string, date: string) => void;
  onRemoveFile: (fileId: string) => void;
}

function DayDropdown({
  group,
  files,
  fileDates,
  customTitles,
  analyzingFiles,
  isUploading,
  onTitleChange,
  onDateChange,
  onRemoveFile,
}: DayDropdownProps) {
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
        <span className="text-base font-semibold">{group.label}</span>
        <span className="text-xs text-muted-foreground ml-auto">
          {group.fileIds.length} {group.fileIds.length === 1 ? 'file' : 'files'}
        </span>
      </button>
      {open && (
        <div className="space-y-2 px-3 pb-3">
          {group.fileIds.map((fileId) => {
            const file = files.find(f => f.id === fileId);
            if (!file) return null;
            return (
              <FileRow
                key={file.id}
                file={file}
                fileDates={fileDates}
                customTitles={customTitles}
                analyzingFiles={analyzingFiles}
                isUploading={isUploading}
                onTitleChange={onTitleChange}
                onDateChange={onDateChange}
                onRemoveFile={onRemoveFile}
              />
            );
          })}
        </div>
      )}
    </div>
  );
}

interface FileRowProps {
  file: UppyFile<UploadMeta, RecordingBody>;
  fileDates: Record<string, string>;
  customTitles: Record<string, string>;
  analyzingFiles: Set<string>;
  isUploading: boolean;
  onTitleChange: (fileId: string, title: string) => void;
  onDateChange: (fileId: string, date: string) => void;
  onRemoveFile: (fileId: string) => void;
}

function FileRow({
  file,
  fileDates,
  customTitles,
  analyzingFiles,
  isUploading,
  onTitleChange,
  onDateChange,
  onRemoveFile,
}: FileRowProps) {
  const isAnalyzing = analyzingFiles.has(file.id);

  return (
    <div className="flex items-center gap-3 p-3 bg-card border rounded-lg group relative">
      <div className="bg-muted w-10 h-10 rounded-md flex items-center justify-center shrink-0">
        <Music className="w-5 h-5 text-muted-foreground" />
      </div>
      
      <div className="flex-1 min-w-0">
        {isUploading ? (
          <div className="font-medium text-sm truncate">
            {customTitles[file.id] || file.name}
          </div>
        ) : isAnalyzing ? (
          <div className="flex items-center gap-2 h-7">
            <Loader2 className="w-3.5 h-3.5 animate-spin text-muted-foreground" />
            <span className="text-sm text-muted-foreground">Analyzing...</span>
          </div>
        ) : (
          <Input
            value={customTitles[file.id] ?? ''}
            onChange={(e) => onTitleChange(file.id, e.target.value)}
            className="h-7 text-sm font-medium border-transparent hover:border-input focus:border-input px-1 -ml-1 bg-transparent"
            placeholder="Auto-generated title"
          />
        )}
        <div className="flex items-center gap-2 mt-1">
          <span className="text-xs text-muted-foreground truncate">
            {file.name}
          </span>
          <span className="text-xs text-muted-foreground">·</span>
          <span className="text-xs text-muted-foreground whitespace-nowrap">
            {formatFileSize(file.size ?? 0)}
          </span>
          {isUploading && file.progress && (
            <span className="text-xs text-primary font-medium">
              {file.progress.percentage}%
            </span>
          )}
        </div>
        {!isUploading && !isAnalyzing && (
          <div className="flex items-center gap-2 mt-1">
            <Input
              type="date"
              value={fileDates[file.id] || ''}
              onChange={(e) => onDateChange(file.id, e.target.value)}
              className="h-7 w-auto text-xs border-transparent hover:border-input focus:border-input px-1 -ml-1 bg-transparent"
            />
            {fileDates[file.id] && fileDates[file.id] !== new Date().toISOString().split('T')[0] && (
              <span className="text-[10px] text-muted-foreground bg-muted px-1.5 py-0.5 rounded-md">
                from metadata
              </span>
            )}
          </div>
        )}
      </div>

      {!isUploading && (
        <Button
          variant="ghost"
          size="icon"
          className="h-8 w-8 text-muted-foreground opacity-0 group-hover:opacity-100 transition-opacity absolute right-2 top-1/2 -translate-y-1/2"
          onClick={() => onRemoveFile(file.id)}
        >
          <X className="w-4 h-4" />
        </Button>
      )}

      {isUploading && file.progress && (
        <div 
          className="absolute bottom-0 left-0 h-1 bg-primary rounded-b-lg transition-all duration-300"
          style={{ width: `${file.progress.percentage}%` }}
        />
      )}
    </div>
  );
}
