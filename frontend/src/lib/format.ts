export function formatBytes(n: number): string {
  if (!Number.isFinite(n) || n < 0) return '—';
  if (n < 1024) return `${n} B`;
  const units = ['KB', 'MB', 'GB', 'TB'];
  let v = n / 1024;
  let i = 0;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i++;
  }
  return `${v < 10 ? v.toFixed(1) : Math.round(v)} ${units[i]}`;
}

/** Splits a rate into a value and unit for large telemetry readouts. */
export function splitRate(bytesPerSec: number): [string, string] {
  if (!bytesPerSec || bytesPerSec < 1) return ['0.0', 'MB/s'];
  const mb = bytesPerSec / 1048576;
  if (mb >= 1) return [mb >= 100 ? mb.toFixed(0) : mb.toFixed(1), 'MB/s'];
  return [(bytesPerSec / 1024).toFixed(0), 'KB/s'];
}

export function formatEta(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds <= 0) return '—';
  if (seconds < 60) return `${Math.ceil(seconds)}s`;
  const m = Math.floor(seconds / 60);
  const s = Math.round(seconds % 60);
  return m < 60 ? `${m}m ${s.toString().padStart(2, '0')}s` : `${Math.floor(m / 60)}h ${m % 60}m`;
}

export function timeAgo(ts: number, now = Date.now()): string {
  const s = Math.max(0, Math.round((now - ts) / 1000));
  if (s < 45) return 'just now';
  if (s < 3600) return `${Math.round(s / 60)}m ago`;
  if (s < 86400) return `${Math.round(s / 3600)}h ago`;
  return new Date(ts).toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
}
