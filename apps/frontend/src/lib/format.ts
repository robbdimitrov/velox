export function formatDurationMs(ms: number): string {
  const abs = Math.abs(ms);
  if (abs < 1000) return `${ms}ms`;
  if (abs < 60_000) return `${(ms / 1000).toFixed(1)}s`;
  if (abs < 3_600_000) return `${(ms / 60_000).toFixed(1)}m`;
  return `${(ms / 3_600_000).toFixed(1)}h`;
}

const freshLagMs = 5000;
const staleLagMs = 120_000;

export function lagToneClass(ms: number): string {
  if (ms <= freshLagMs) return 'text-ok';
  if (ms <= staleLagMs) return 'text-warn';
  return 'text-urgency';
}
