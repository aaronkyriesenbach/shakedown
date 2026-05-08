import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { apiFetch } from '@/api/client';

export interface User {
  id: string;
  oidc_sub: string;
  email: string;
  display_name: string;
  avatar_url?: string;
  role: 'user' | 'admin';
  created_at: string;
  updated_at: string;
}

export const authKeys = {
  me: () => ['auth', 'me'] as const,
};

export function useMe() {
  return useQuery({
    queryKey: authKeys.me(),
    queryFn: () => apiFetch<User>('/api/auth/me'),
    retry: false,
    staleTime: 5 * 60 * 1000, // 5 minutes
  });
}

export function useLogout() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () =>
      apiFetch<void>('/api/auth/logout', { method: 'POST' }),
    onSuccess: () => {
      queryClient.clear();
      window.location.href = '/login';
    },
  });
}

export function loginUrl(): string {
  return '/api/auth/login';
}
