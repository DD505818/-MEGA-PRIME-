'use client';

import { useWebSocket } from '@/hooks/useWebSocket';
import { useSystemStore } from '@/lib/stores/systemStore';
import { modeToBadge } from '@/lib/utils/theme';

export function TopBar() {
  const { connected } = useWebSocket();
  const mode = useSystemStore((s) => s.mode);
  return (
    <header className="flex items-center justify-between border-b border-white/10 px-4 py-3">
      <div className="text-sm">Operator Console</div>
      <div className="flex items-center gap-3 text-xs">
        <span className={`rounded px-2 py-1 ${modeToBadge(mode)}`}>{mode.toUpperCase()}</span>
        <span className={connected ? 'text-green-400' : 'text-red-400'}>{connected ? 'STREAM LIVE' : 'DISCONNECTED'}</span>
      </div>
    </header>
  );
}
