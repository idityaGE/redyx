/**
 * Relative time formatting for timestamps.
 * Converts ISO timestamp strings to compact relative time (2h, 3d, 5mo).
 */

export function relativeTime(isoString: string): string {
  if (!isoString) return '';
  const then = new Date(isoString).getTime();
  if (isNaN(then)) return '';
  // Treat Go zero time (year 1) and Unix epoch as missing timestamps
  if (then < 0 || then === 0) return '';

  const now = Date.now();
  const seconds = Math.floor((now - then) / 1000);

  if (seconds < 60) return 'just now';
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h`;
  const days = Math.floor(hours / 24);
  if (days < 30) return `${days}d`;
  const months = Math.floor(days / 30);
  if (months < 12) return `${months}mo`;
  const years = Math.floor(days / 365);
  return `${years}y`;
}
