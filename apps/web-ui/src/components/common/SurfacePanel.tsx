'use client';

import { EmptyState } from './EmptyState';

export function isEmptyPayload(data: unknown): boolean {
  if (data === null || data === undefined) return true;
  if (Array.isArray(data)) return data.length === 0;
  if (typeof data === 'object') {
    const rec = data as Record<string, unknown>;
    // Legacy stub shape { data: [] } counts as empty.
    if (Array.isArray(rec.data)) return rec.data.length === 0;
    return Object.keys(rec).length === 0;
  }
  return false;
}

interface SurfacePanelProps {
  title: string;
  data: unknown;
  isLoading: boolean;
  isError: boolean;
}

/**
 * Standard surface state: explicit "Unavailable — backend unreachable or not
 * configured" on error/empty via EmptyState. On success it shows connection
 * state only — payload contents are displayed by dedicated components, and
 * nothing is ever fabricated.
 */
export function SurfacePanel({ title, data, isLoading, isError }: SurfacePanelProps) {
  if (isLoading) {
    return <div className="panel text-sm text-white/60">Loading {title.toLowerCase()}…</div>;
  }
  if (isError || isEmptyPayload(data)) {
    return (
      <EmptyState title={title} message="Unavailable — backend unreachable or not configured." />
    );
  }
  return (
    <div className="panel text-sm">
      <p className="font-semibold">{title}</p>
      <p className="mt-1 text-white/60">Connected — upstream responded.</p>
    </div>
  );
}
