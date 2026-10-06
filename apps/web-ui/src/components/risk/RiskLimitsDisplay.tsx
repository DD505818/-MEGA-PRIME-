'use client';

import { useRisk } from '@/hooks/useRisk';
import { EmptyState } from '@/components/common/EmptyState';

/**
 * Read-only risk limits display.
 *
 * Values are fetched from /api/risk and displayed as received. This UI does
 * not edit limits: limit changes require the backend/operator path, not this UI.
 * Unreachable or empty state renders as an explicit unavailable message —
 * never synthetic numbers.
 */
export function RiskLimitsDisplay() {
  const { data, isError, isLoading } = useRisk();

  if (isLoading) {
    return <div className="panel text-sm text-white/60">Loading risk limits…</div>;
  }

  const values =
    !isError && data && typeof data === 'object' && !Array.isArray(data)
      ? (data as Record<string, unknown>)
      : null;

  if (!values || Object.keys(values).length === 0) {
    return (
      <EmptyState
        title="Risk limits"
        message="Unavailable — backend unreachable or not configured."
      />
    );
  }

  return (
    <div className="panel space-y-3">
      <h2 className="text-sm font-semibold uppercase tracking-wide text-white/60">Risk limits</h2>
      <dl className="grid grid-cols-1 gap-2 sm:grid-cols-2">
        {Object.entries(values).map(([key, value]) => (
          <div key={key} className="rounded border border-white/10 bg-white/5 px-3 py-2">
            <dt className="text-xs text-white/50">{key}</dt>
            <dd className="text-sm">
              {value === null || value === undefined
                ? 'UNKNOWN'
                : typeof value === 'object'
                  ? JSON.stringify(value).slice(0, 200)
                  : String(value)}
            </dd>
          </div>
        ))}
      </dl>
      <p className="text-xs text-white/50">
        Read-only. Limit changes require the backend/operator path, not this UI.
      </p>
    </div>
  );
}
