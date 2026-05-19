/**
 * Cross-browser media helpers.
 *
 * iOS Safari's HTMLMediaElement can get stuck at readyState 2 (HAVE_CURRENT_DATA)
 * when paused, which causes `currentTime` assignments to silently fail. When
 * `play()` is called afterward the engine resets to position 0.
 *
 * `fastSeek()` is a Safari-native API that bypasses this readyState gate and is
 * widely used by Video.js, Mastodon, and Koel for reliable seeking.
 */

/**
 * Seek a media element using `fastSeek()` when available (Safari), falling back
 * to a direct `currentTime` assignment on all other browsers.
 */
export function safeSeek(el: HTMLMediaElement, seconds: number): void {
  const clamped = Math.max(0, Math.min(seconds, el.duration || 0));
  if (typeof el.fastSeek === 'function') {
    el.fastSeek(clamped);
  } else {
    el.currentTime = clamped;
  }
}
