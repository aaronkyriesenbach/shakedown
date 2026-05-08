import { useCallback, useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import Uppy, { type Meta, type Body } from '@uppy/core';
import type { UploadResult, UppyFile } from '@uppy/core';
import XHRUpload from '@uppy/xhr-upload';
import { toast } from 'sonner';
import { Upload, X, Music, Loader2 } from 'lucide-react';

import { Label } from '@/components/ui/label';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { formatFileSize } from '@/lib/format';

interface UploadMeta extends Meta {
  title?: string;
  recorded_at?: string;
}

interface RecordingBody extends Body {
  id: string;
  title: string;
  playback_ready: boolean;
}

export function UploadForm() {
  const navigate = useNavigate();
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [recordedAt, setRecordedAt] = useState<string>(() => {
    const today = new Date();
    return today.toISOString().split('T')[0];
  });
  const [files, setFiles] = useState<UppyFile<UploadMeta, RecordingBody>[]>([]);
  const [fileTitles, setFileTitles] = useState<Record<string, string>>({});
  const [isDragging, setIsDragging] = useState(false);
  const [isUploading, setIsUploading] = useState(false);

  const [uppy] = useState(() => {
    const u = new Uppy<UploadMeta, RecordingBody>({
      restrictions: {
        allowedFileTypes: ['audio/*'],
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

      successful.forEach((file: UppyFile<UploadMeta, RecordingBody>) => {
        toast.success(`Uploaded ${file.name} successfully`);
        const id = file.response?.body?.id;
        if (id) {
          navigate(`/recordings/${id}`);
        }
      });

      failed.forEach((file: UppyFile<UploadMeta, RecordingBody>) => {
        toast.error(`Failed to upload ${file.name}`);
      });
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
  }, [uppy, navigate, syncFiles]);

  useEffect(() => {
    uppy.setMeta({ recorded_at: recordedAt });
  }, [uppy, recordedAt]);

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

  const handleDrop = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(false);
    if (e.dataTransfer.files.length > 0) {
      addFiles(e.dataTransfer.files);
    }
  }, [addFiles]);

  const handleDragOver = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(true);
  }, []);

  const handleDragLeave = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(false);
  }, []);

  const handleFileInput = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files && e.target.files.length > 0) {
      addFiles(e.target.files);
      e.target.value = '';
    }
  }, [addFiles]);

  const handleTitleChange = (fileId: string, value: string) => {
    setFileTitles((prev) => ({ ...prev, [fileId]: value }));
    uppy.setFileMeta(fileId, { title: value.trim() });
  };

  const handleRemoveFile = (fileId: string) => {
    uppy.removeFile(fileId);
    setFileTitles((prev) => {
      const next = { ...prev };
      delete next[fileId];
      return next;
    });
  };

  const handleUpload = () => {
    uppy.upload();
  };

  const openFilePicker = () => {
    fileInputRef.current?.click();
  };

  return (
    <div className="space-y-6">
      <div className="space-y-2">
        <Label htmlFor="recorded_at">Recording Date</Label>
        <Input
          id="recorded_at"
          type="date"
          value={recordedAt}
          onChange={(e) => setRecordedAt(e.target.value)}
          className="max-w-xs"
        />
      </div>

      <input
        ref={fileInputRef}
        type="file"
        accept="audio/*"
        multiple
        onChange={handleFileInput}
        className="sr-only"
      />

      {files.length === 0 ? (
        <div
          role="button"
          tabIndex={0}
          onClick={openFilePicker}
          onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') openFilePicker(); }}
          onDrop={handleDrop}
          onDragOver={handleDragOver}
          onDragLeave={handleDragLeave}
          className={`
            relative flex flex-col items-center justify-center rounded-lg border-2 border-dashed p-12
            cursor-pointer transition-colors
            ${isDragging
              ? 'border-primary bg-primary/5'
              : 'border-muted-foreground/25 hover:border-primary/50 hover:bg-muted/50'
            }
          `}
        >
          <Upload className="w-10 h-10 text-muted-foreground mb-4" />
          <p className="text-sm font-medium">
            Drop audio files here or click to browse
          </p>
          <p className="text-xs text-muted-foreground mt-1">
            Supports MP3, WAV, FLAC, and other audio formats
          </p>
        </div>
      ) : (
        <div
          onDrop={handleDrop}
          onDragOver={handleDragOver}
          onDragLeave={handleDragLeave}
          className="space-y-3"
        >
          {files.map((file) => {
            const progress = file.progress;
            const isFileUploading = progress?.uploadStarted && !progress?.uploadComplete;
            const bytesUploaded: number = Number(progress?.bytesUploaded) || 0;
            const bytesTotal: number = Number(progress?.bytesTotal) || 0;
            const uploadPercent = bytesTotal > 0
              ? Math.round((bytesUploaded / bytesTotal) * 100)
              : 0;

            return (
              <div key={file.id} className="rounded-lg border bg-card p-4 space-y-3">
                <div className="flex items-center gap-3">
                  <div className="bg-muted w-9 h-9 rounded-md flex items-center justify-center shrink-0 text-muted-foreground">
                    <Music className="w-4 h-4" />
                  </div>
                  <div className="min-w-0 flex-1">
                    <p className="text-sm font-medium truncate">{file.name}</p>
                    <p className="text-xs text-muted-foreground">
                      {formatFileSize(file.size ?? 0)}
                    </p>
                  </div>
                  {!isUploading && (
                    <button
                      type="button"
                      onClick={() => handleRemoveFile(file.id)}
                      className="text-muted-foreground hover:text-destructive transition-colors p-1 rounded-md hover:bg-destructive/10"
                    >
                      <X className="w-4 h-4" />
                    </button>
                  )}
                </div>

                {isFileUploading && (
                  <div className="w-full bg-muted rounded-full h-1.5 overflow-hidden">
                    <div
                      className="bg-primary h-full rounded-full transition-all duration-300"
                      style={{ width: `${uploadPercent}%` }}
                    />
                  </div>
                )}

                {!isUploading && (
                  <Input
                    type="text"
                    value={fileTitles[file.id] ?? ''}
                    onChange={(e) => handleTitleChange(file.id, e.target.value)}
                    placeholder="Recording name (leave blank for auto-generated)"
                  />
                )}
              </div>
            );
          })}

          <div className="flex items-center gap-3 pt-2">
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={isUploading}
              onClick={openFilePicker}
            >
              Add more files
            </Button>

            <div className="flex-1" />

            <Button
              type="button"
              onClick={handleUpload}
              disabled={isUploading || files.length === 0}
            >
              {isUploading ? (
                <>
                  <Loader2 className="w-4 h-4 animate-spin" />
                  Uploading…
                </>
              ) : (
                <>
                  <Upload className="w-4 h-4" />
                  Upload {files.length > 1 ? `${files.length} files` : 'file'}
                </>
              )}
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
