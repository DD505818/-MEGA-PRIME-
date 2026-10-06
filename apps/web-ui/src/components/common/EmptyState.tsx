/**
 * EmptyState — renders when a surface has no data to show.
 *
 * Per doctrine, unavailable backend state must be explicit ("Unavailable —
 * backend unreachable or not configured"), never filled with synthetic data.
 */
export function EmptyState({ title = 'No data', message }: { title?: string; message?: string }) {
  return (
    <div className="panel text-sm">
      <p className="font-semibold">{title}</p>
      {message ? <p className="mt-1 text-white/60">{message}</p> : null}
    </div>
  );
}
