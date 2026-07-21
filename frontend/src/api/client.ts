const BASE_URL = import.meta.env.VITE_API_URL ?? '';

export class ApiError extends Error {
  public readonly status: number;
  public readonly body: string;
  
  constructor(
    status: number,
    message: string,
    body: string,
  ) {
    super(message);
    this.status = status;
    this.body = body;
    this.name = 'ApiError';
  }

  get userMessage(): string {
    try {
      const parsed = JSON.parse(this.body);
      if (typeof parsed.error === 'string') return parsed.error;
    } catch {}
    return this.body || this.message;
  }
}

export async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE_URL}${path}`, {
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      ...init?.headers,
    },
    ...init,
  });

  if (!res.ok) {
    const text = await res.text().catch(() => 'Unknown error');
    throw new ApiError(res.status, `API ${res.status}: ${text}`, text);
  }

  // Handle empty response bodies (e.g. 204 No Content, or 202 Accepted with
  // no body). Read as text first since an empty string is not valid JSON.
  const text = await res.text();
  if (!text) {
    return undefined as T;
  }

  return JSON.parse(text) as T;
}
