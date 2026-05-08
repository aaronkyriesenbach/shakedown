import { useMe } from '@/api/auth';

export function useAuth() {
  const { data: user, isLoading, isError } = useMe();

  return {
    user: user ?? null,
    isLoading,
    isAuthenticated: !!user && !isError,
    isAdmin: user?.role === 'admin',
  };
}