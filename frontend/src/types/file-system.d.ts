/**
 * React JSX type augmentation for the `webkitdirectory` input attribute.
 *
 * WHY THIS EXISTS:
 * TypeScript's DOM lib includes `HTMLInputElement.webkitdirectory: boolean` for
 * direct DOM access, but React's `InputHTMLAttributes` (in @types/react) does
 * not include it in the JSX attribute type map. This is a long-standing gap —
 * React's DOM runtime correctly renders the attribute when passed as a string,
 * but the types reject it at compile time.
 *
 * The value must be a string (e.g. `webkitdirectory=""`) in JSX, not a boolean,
 * because React only renders boolean-to-attribute conversion for its known
 * whitelisted properties.
 *
 * WHEN TO REMOVE:
 * Delete this file once @types/react adds `webkitdirectory` to
 * `InputHTMLAttributes`. Track the upstream issue:
 *   https://github.com/facebook/react/issues/3468
 *
 * REFERENCES:
 * - React issue (open since 2015): https://github.com/facebook/react/issues/3468
 * - MDN spec: https://developer.mozilla.org/en-US/docs/Web/API/HTMLInputElement/webkitdirectory
 * - Reached "Baseline Newly Available" August 2025: https://web-platform-dx.github.io/web-features-explorer/features/input-file-webkitdirectory/
 * - eslint-plugin-react recognizes it: https://github.com/jsx-eslint/eslint-plugin-react/issues/3454
 * - Widely-used workaround across OSS (Ollama, Koodo Reader, ILLA Builder, etc.)
 */
import 'react';

declare module 'react' {
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  interface InputHTMLAttributes<T> {
    webkitdirectory?: string;
    directory?: string;
  }
}
