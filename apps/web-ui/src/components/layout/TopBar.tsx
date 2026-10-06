'use client';

import { useStatus } from '@/hooks/useStatus';
import { useWebSocket } from '@/hooks/useWebSocket';
import { modeToBadge } from '@/lib/utils/theme';

/**
 * Top bar. Mode is read from /api/status — the only mode truth the UI trusts.
 * A local store default is never used: anything but an explicit PAPER
 * response renders UNKNOWN.
 */
export function TopBar() {
  const { connected } = useWebSocket();
  const { data, isError } = useStatus();
  const mode = isError || !data ? 'UNKNOWN' : data.mode;

  return (
    <header className="flex items-center justify-between border-b border-white/10 px-4 py-3">
      <div className="text-sm">Operator Console</div>
      <div className="flex items-center gap-3 text-xs">
        <span className={`rounded px-2 py-1 ${modeToBadge(mode)}`}>{mode}</span>
        <span className={connected ? 'text-green-400' : 'text-red-400'}>
          {connected ? 'STREAM LIVE' : 'DISCONNECTED'}
        </span>
      </div>
    </header>
  );
}
