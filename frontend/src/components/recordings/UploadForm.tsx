import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import Uppy, { type Meta, type Body } from '@uppy/core';
import type { UploadResult, UppyFile } from '@uppy/core';
import Dashboard from '@uppy/react/dashboard';
import XHRUpload from '@uppy/xhr-upload';
import { toast } from 'sonner';

import '@uppy/core/css/style.min.css';
import '@uppy/dashboard/css/style.min.css';

import { Label } from '@/components/ui/label';
import { Input } from '@/components/ui/input';

interface UploadMeta extends Meta {
  recorded_at: string;
}

interface RecordingBody extends Body {
  id: string;
  title: string;
  playback_ready: boolean;
}

export function UploadForm() {
  const navigate = useNavigate();
  const [recordedAt, setRecordedAt] = useState<string>(() => {
    const today = new Date();
    return today.toISOString().split('T')[0];
  });

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

  useEffect(() => {
    uppy.setMeta({ recorded_at: recordedAt });
  }, [uppy, recordedAt]);

  useEffect(() => {
    const handleComplete = (result: UploadResult<UploadMeta, RecordingBody>) => {
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

    uppy.on('complete', handleComplete);
    return () => {
      uppy.off('complete', handleComplete);
    };
  }, [uppy, navigate]);

  const isDarkMode = document.documentElement.classList.contains('dark') || 
                     window.matchMedia('(prefers-color-scheme: dark)').matches;

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

      <div className="uppy-container" style={{ '--uppy-accent-color': '#6366f1' } as React.CSSProperties}>
        <Dashboard
          uppy={uppy}
          theme={isDarkMode ? 'dark' : 'light'}
          width="100%"
          height={400}
          proudlyDisplayPoweredByUppy={false}
          hideUploadButton={false}
        />
      </div>
    </div>
  );
}
