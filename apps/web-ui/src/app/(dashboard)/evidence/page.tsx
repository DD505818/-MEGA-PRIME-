'use client';

import { useQuery } from '@tanstack/react-query';
import { api } from '@/lib/api';
import { PageHeader } from '@/components/layout/PageHeader';
import { EmptyState } from '@/components/common/EmptyState';
import { FUNNEL_STAGES, type EvidencePayload } from '@/lib/evidence-types';

const useEvidence = () =>
  useQuery({
    queryKey: ['evidence'],
    queryFn: async () => (await api.get('/evidence')).data as EvidencePayload
  });

const FUNNEL_LABELS: Record<string, string> = {
  costs: 'Stopped at costs (after-cost edge)',
  walkforward: 'Stopped at walk-forward',
  nulls: 'Stopped at null tests',
  montecarlo: 'Stopped at Monte Carlo',
  overfit: 'Stopped at overfit controls'
};

/**
 * Edge-campaign evidence.
 *
 * Read-only rendering of the frozen campaign artifacts served by
 * /api/evidence. The funnel counts and per-configuration rows are computed
 * from real lab reports at request time — nothing is hardcoded and nothing
 * is fabricated. When evidence is unavailable this renders UNKNOWN.
 */
export default function EvidencePage() {
  const { data, isError, isLoading } = useEvidence();

  if (isLoading) {
    return (
      <section className="space-y-4">
        <PageHeader title="Evidence" subtitle="Edge-campaign evidence" />
        <div className="panel text-sm text-white/60">Loading evidence…</div>
      </section>
    );
  }

  if (isError || !data || !data.available) {
    return (
      <section className="space-y-4">
        <PageHeader title="Evidence" subtitle="Edge-campaign evidence" />
        <EmptyState
          title="Evidence unavailable"
          message="UNKNOWN — the evidence mount (EVIDENCE_DIR) is absent or unreadable. No campaign data is displayed rather than fabricated."
        />
      </section>
    );
  }

  const total = FUNNEL_STAGES.reduce((sum, s) => sum + (data.funnel[s] ?? 0), 0);

  return (
    <section className="space-y-6">
      <PageHeader
        title="Evidence"
        subtitle="Kill funnel and per-configuration results from the frozen edge campaigns."
      />

      <div className="panel">
        <h2 className="mb-2 text-sm font-semibold uppercase tracking-wide text-white/60">
          Kill funnel ({total} configurations evaluated)
        </h2>
        <div className="space-y-2">
          {FUNNEL_STAGES.map((stage) => {
            const count = data.funnel[stage] ?? 0;
            const pct = total > 0 ? Math.round((count / total) * 100) : 0;
            return (
              <div key={stage} className="flex items-center gap-3 text-sm">
                <span className="w-64 shrink-0 text-white/70">{FUNNEL_LABELS[stage]}</span>
                <div className="h-2 flex-1 rounded bg-white/10">
                  <div className="h-2 rounded bg-amber-500/70" style={{ width: `${pct}%` }} />
                </div>
                <span className="w-20 shrink-0 text-right font-mono">
                  {count} ({pct}%)
                </span>
              </div>
            );
          })}
        </div>
      </div>

      <div className="panel">
        <h2 className="mb-2 text-sm font-semibold uppercase tracking-wide text-white/60">
          Campaign records
        </h2>
        <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
          {data.campaigns.map((c) => (
            <div key={c.campaign} className="rounded border border-white/10 bg-white/5 p-3 text-sm">
              <p className="font-semibold">Campaign #{c.campaign}</p>
              <p className="text-white/60">Configurations evaluated: {c.configsEvaluated}</p>
              <p className="text-white/60">Verdict: {c.verdict}</p>
              <p className="text-white/60">Qualifier: {c.qualifier ?? 'none'}</p>
            </div>
          ))}
        </div>
      </div>

      <div className="panel">
        <h2 className="mb-2 text-sm font-semibold uppercase tracking-wide text-white/60">
          Per-configuration stage table
        </h2>
        <div className="overflow-x-auto">
          <table className="w-full text-left text-sm">
            <thead>
              <tr className="text-xs uppercase text-white/50">
                <th className="py-1 pr-4">#</th>
                <th className="py-1 pr-4">Campaign</th>
                <th className="py-1 pr-4">Family</th>
                <th className="py-1 pr-4">Candidate</th>
                <th className="py-1 pr-4">Stage reached</th>
                <th className="py-1">Verdict</th>
              </tr>
            </thead>
            <tbody>
              {data.configs.map((c) => (
                <tr key={`${c.campaign}-${c.number}`} className="border-t border-white/10">
                  <td className="py-1 pr-4 font-mono">{c.number}</td>
                  <td className="py-1 pr-4">{c.campaign}</td>
                  <td className="py-1 pr-4">{c.family}</td>
                  <td className="py-1 pr-4 font-mono text-xs">{c.candidate}</td>
                  <td className="py-1 pr-4">{c.stageReached ?? 'passed'}</td>
                  <td className="py-1">{c.verdict}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      <div className="panel space-y-2 text-sm text-white/60">
        <p>
          <span className="font-semibold text-white">Monte Carlo v2:</span> {data.mcV2Status}.
        </p>
        <p>
          <span className="font-semibold text-white">Stage-7 wall:</span> open design question for the
          Campaign #3 preregistration — whether stage 7 (sensitivity, DSR, minimum track length) is
          the intended wall for what the v2 Monte Carlo lets through.
        </p>
      </div>
    </section>
  );
}
