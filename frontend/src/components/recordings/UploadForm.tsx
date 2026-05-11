import { useCallback, useEffect, useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import Uppy, { type Meta, type Body } from '@uppy/core';
import type { UploadResult, UppyFile } from '@uppy/core';
import XHRUpload from '@uppy/xhr-upload';
import { toast } from 'sonner';
import { Upload, X, Music, Loader2, CheckCircle2, AlertCircle, RefreshCw } from 'lucide-react';

import { Badge } from '@/components/ui/badge';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { formatFileSize } from '@/lib/format';
import { apiFetch } from '@/api/client';
import type { Recording } from '@/api/recordings';

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

interface ProbeResult {
  title_preview: string;
  recorded_at: string;
  next_number: number;
  date_source: string;
}

export function UploadForm() {
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [fileDates, setFileDates] = useState<Record<string, string>>({});
  const [files, setFiles] = useState<UppyFile<UploadMeta, RecordingBody>[]>([]);
  const [fileTitles, setFileTitles] = useState<Record<string, string>>({});
  const [isDragging, setIsDragging] = useState(false);
  const [isUploading, setIsUploading] = useState(false);
  const [uploadResults, setUploadResults] = useState<UploadResultType[] | null>(null);
  const [polledRecordings, setPolledRecordings] = useState<Record<string, Recording>>({});
  const [probeResults, setProbeResults] = useState<Record<string, ProbeResult>>({});
  const [probingFiles, setProbingFiles] = useState<Set<string>>(new Set());
  const probedFilesRef = useRef<Set<string>>(new Set());

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

  const probeFile = useCallback(async (file: UppyFile<UploadMeta, RecordingBody>, fallbackDate: string, offset: number) => {
    setProbingFiles(prev => {
      const next = new Set(prev);
      next.add(file.id);
      return next;
    });

    try {
      const formData = new FormData();
      formData.append('file', file.data as Blob, file.name);
      formData.append('fallback_date', fallbackDate);
      if (offset > 0) {
        formData.append('offset', String(offset));
      }

      const res = await fetch('/api/recordings/probe', {
        method: 'POST',
        body: formData,
        credentials: 'include',
      });

      if (!res.ok) return;

      const result: ProbeResult = await res.json();
      setProbeResults(prev => ({ ...prev, [file.id]: result }));

      setFileDates(prev => {
        if (prev[file.id] === undefined) {
          uppy.setFileMeta(file.id, { recorded_at: result.recorded_at });
          return { ...prev, [file.id]: result.recorded_at };
        }
        return prev;
      });

      setFileTitles(prev => {
        if (prev[file.id] === undefined || prev[file.id] === '') {
          uppy.setFileMeta(file.id, { title: result.title_preview });
          return { ...prev, [file.id]: result.title_preview };
        }
        return prev;
      });
    } catch {
      // Probe failed — title will be auto-generated by backend on upload
    } finally {
      setProbingFiles(prev => {
        const next = new Set(prev);
        next.delete(file.id);
        return next;
      });
    }
  }, [uppy]);

  useEffect(() => {
    let offset = 0;
    const today = new Date().toISOString().split('T')[0];
    for (const file of files) {
      if (!probedFilesRef.current.has(file.id)) {
        probedFilesRef.current.add(file.id);
        probeFile(file, today, offset);
        offset++;
      }
    }
  }, [files, probeFile]);

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

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(false);
    if (e.dataTransfer.files?.length > 0) {
      addFiles(e.dataTransfer.files);
    }
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
    setFileTitles({});
    setFileDates({});
    setProbeResults({});
    setProbingFiles(new Set());
    probedFilesRef.current.clear();
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
              // Reset input so the same file can be selected again
              e.target.value = '';
            }}
          />
          
          <div className="mx-auto w-12 h-12 mb-4 bg-muted rounded-full flex items-center justify-center">
            <Upload className="w-6 h-6 text-muted-foreground" />
          </div>
          
          <h3 className="text-lg font-semibold mb-1">
            Drag & drop audio or video files
          </h3>
          <p className="text-sm text-muted-foreground mb-4">
            or click to browse from your computer
          </p>
          
          <Button
            type="button"
            variant="secondary"
            onClick={() => fileInputRef.current?.click()}
            disabled={isUploading}
          >
            Select Files
          </Button>
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
                  setFileTitles({});
                  setFileDates({});
                  setProbeResults({});
                  setProbingFiles(new Set());
                  probedFilesRef.current.clear();
                }}
              >
                Clear all
              </Button>
            )}
          </div>
          
          <div className="space-y-2">
            {files.map((file) => {
              const isProbing = probingFiles.has(file.id);
              const probe = probeResults[file.id];

              return (
                <div
                  key={file.id}
                  className="flex items-center gap-3 p-3 bg-card border rounded-lg group relative"
                >
                  <div className="bg-muted w-10 h-10 rounded-md flex items-center justify-center shrink-0">
                    <Music className="w-5 h-5 text-muted-foreground" />
                  </div>
                  
                  <div className="flex-1 min-w-0">
                    {isUploading ? (
                      <div className="font-medium text-sm truncate">
                        {fileTitles[file.id] || file.name}
                      </div>
                    ) : isProbing ? (
                      <div className="flex items-center gap-2 h-7">
                        <Loader2 className="w-3.5 h-3.5 animate-spin text-muted-foreground" />
                        <span className="text-sm text-muted-foreground">Analyzing...</span>
                      </div>
                    ) : (
                      <Input
                        value={fileTitles[file.id] ?? ''}
                        onChange={(e) => {
                          const newTitle = e.target.value;
                          setFileTitles(prev => ({ ...prev, [file.id]: newTitle }));
                          uppy.setFileMeta(file.id, { title: newTitle.trim() });
                        }}
                        className="h-7 text-sm font-medium border-transparent hover:border-input focus:border-input px-1 -ml-1 bg-transparent"
                        placeholder={probe?.title_preview ?? 'Enter a title'}
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
                    {!isUploading && !isProbing && (
                      <div className="flex items-center gap-2 mt-1">
                        <Input
                          type="date"
                          value={fileDates[file.id] || ''}
                          onChange={(e) => {
                            const newDate = e.target.value;
                            setFileDates(prev => ({ ...prev, [file.id]: newDate }));
                            uppy.setFileMeta(file.id, { recorded_at: newDate });
                            probedFilesRef.current.delete(file.id);
                            
                            setProbeResults(prev => {
                              const next = { ...prev };
                              delete next[file.id];
                              return next;
                            });
                            setFileTitles(prev => {
                              const next = { ...prev };
                              delete next[file.id];
                              return next;
                            });
                            
                            probeFile(file, newDate, 0);
                          }}
                          className="h-7 w-auto text-xs border-transparent hover:border-input focus:border-input px-1 -ml-1 bg-transparent"
                        />
                        {probe?.date_source === 'embedded_tags' && (
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
                      onClick={() => {
                        uppy.removeFile(file.id);
                        setFileTitles(prev => {
                          const next = { ...prev };
                          delete next[file.id];
                          return next;
                        });
                        setFileDates(prev => {
                          const next = { ...prev };
                          delete next[file.id];
                          return next;
                        });
                        setProbeResults(prev => {
                          const next = { ...prev };
                          delete next[file.id];
                          return next;
                        });
                        probedFilesRef.current.delete(file.id);
                      }}
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
            })}
          </div>

          <Button
            className="w-full"
            size="lg"
            onClick={() => uppy.upload()}
            disabled={isUploading || probingFiles.size > 0}
          >
            {isUploading ? (
              <>
                <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                Uploading...
              </>
            ) : probingFiles.size > 0 ? (
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
