'use client';

import { useRisk } from '@/hooks/useRisk';

/**
 * Read-only kill-switch status display.
 *
 * Kill status may be DISPLAYED by this UI but never triggered from it: this
 * component renders state only, with no buttons, no store actions, and no
 * mutation paths of any kind. Unknown or unreachable state renders UNKNOWN.
 */
export function KillStatusBadge() {
  const { data, isError, isLoading } = useRisk();

  const raw = !isError && data && typeof data === 'object' ? (data as Record<string, unknown>) : null;
  const value = raw
    ? (raw.killSwitch ?? raw.kill_switch ?? raw.killEngaged ?? raw.kill_engaged)
    : undefined;

  let state: 'ENGAGED' | 'NOT ENGAGED' | 'UNKNOWN';
  if (isLoading || isError || !raw) {
    state = 'UNKNOWN';
  } else if (value === true || value === 'engaged' || value === 'active' || value === 'ENGAGED') {
    state = 'ENGAGED';
  } else if (value === false || value === 'disengaged' || value === 'inactive') {
    state = 'NOT ENGAGED';
  } else {
    state = 'UNKNOWN';
  }

  const style =
    state === 'ENGAGED'
      ? 'bg-red-500/20 text-red-200'
      : state === 'NOT ENGAGED'
        ? 'bg-green-500/20 text-green-200'
        : 'bg-white/10 text-white/60';

  return (
    <div className="panel flex items-center justify-between">
      <span className="text-sm text-white/60">Kill switch (read-only)</span>
      <span className={`rounded px-2 py-1 text-xs font-semibold ${style}`}>{state}</span>
    </div>
  );
}
