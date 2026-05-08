import { useEffect } from 'react';
import { Music } from 'lucide-react';
import { loginUrl } from '@/api/auth';

export default function LoginPage() {
  useEffect(() => {
    const timer = setTimeout(() => {
      window.location.href = loginUrl();
    }, 500);
    return () => clearTimeout(timer);
  }, []);

  return (
    <div className="flex min-h-screen items-center justify-center bg-background">
      <div className="text-center">
        <Music className="mx-auto mb-4 h-12 w-12 text-primary" />
        <h1 className="mb-2 text-2xl font-bold">Shakedown</h1>
        <p className="text-muted-foreground">Redirecting to login...</p>
        <div className="mt-4 h-1 w-48 mx-auto bg-secondary rounded-full overflow-hidden">
          <div className="h-full bg-primary animate-pulse rounded-full" />
        </div>
      </div>
    </div>
  );
}
