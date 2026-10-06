'use client';

import { PageHeader } from '@/components/layout/PageHeader';
import { KillStatusBadge } from '@/components/killswitch/KillStatusBadge';
import { useStatus } from '@/hooks/useStatus';
import {
  AEGIS_GATES,
  AUTHORITY_CHAIN,
  LIVE_LOCKED_WORDING,
  PROGRAM_BANNER
} from '@/lib/doctrine';

/**
 * Doctrine status home.
 *
 * Observe / explain / request / display ONLY: no controls, no actions, no
 * execution paths. Program-state banner carries the exact required wording;
 * unknown state renders UNKNOWN.
 */
export default function DashboardHomePage() {
  const { data, isError, isLoading } = useStatus();
  const mode = isError || !data ? 'UNKNOWN' : data.mode;

  return (
    <section className="space-y-6">
      <PageHeader
        title="Operator Status"
        subtitle="Observe · explain · request · display — this UI never executes."
      />

      <div className="panel border-amber-500/30">
        <p className="text-lg font-bold tracking-wide">{PROGRAM_BANNER}</p>
        <p className="mt-1 text-sm text-white/60">{LIVE_LOCKED_WORDING}</p>
        <p className="mt-3 text-xs text-white/60">
          Mode:{' '}
          <span className="rounded bg-white/10 px-2 py-1 font-semibold text-white">
            {isLoading ? '…' : mode}
          </span>
        </p>
      </div>

      <div className="panel">
        <h2 className="mb-2 text-sm font-semibold uppercase tracking-wide text-white/60">
          Authority chain
        </h2>
        <p className="text-sm font-medium">{AUTHORITY_CHAIN.join(' → ')}</p>
      </div>

      <div className="space-y-2">
        <h2 className="text-sm font-semibold uppercase tracking-wide text-white/60">
          AEGIS gates (14)
        </h2>
        <div className="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3">
          {AEGIS_GATES.map((gate) => (
            <div key={gate.n} className="panel">
              <p className="text-xs text-white/50">Gate {gate.n}</p>
              <p className="font-semibold">{gate.name}</p>
              <p className="mt-1 break-all font-mono text-xs text-white/60">{gate.reason}</p>
              <p className="mt-1 text-sm text-white/60">{gate.purpose}</p>
            </div>
          ))}
        </div>
      </div>

      <KillStatusBadge />
    </section>
  );
}
