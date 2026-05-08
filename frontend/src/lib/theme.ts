// Design token exports for use in TypeScript code
// (CSS custom properties handle runtime theming)

export const THEME_STORAGE_KEY = 'shakedown-theme';

export type Theme = 'light' | 'dark' | 'system';
export type ResolvedTheme = 'light' | 'dark';

// Color constants (matching CSS vars for use in canvas/SVG contexts)
export const COLORS = {
  waveformPrimary: {
    light: 'hsl(239, 84%, 67%)',
    dark: 'hsl(239, 84%, 67%)',
  },
  waveformMuted: {
    light: 'hsl(239, 30%, 60%)',
    dark: 'hsl(239, 30%, 40%)',
  },
  waveformProgress: {
    light: 'hsl(239, 84%, 75%)',
    dark: 'hsl(239, 84%, 75%)',
  },
} as const;

// Breakpoints (matching Tailwind defaults)
export const BREAKPOINTS = {
  sm: 640,
  md: 768,
  lg: 1024,
  xl: 1280,
} as const;