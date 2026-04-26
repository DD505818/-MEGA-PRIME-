export const modeToBadge = (mode: string) =>
  mode === 'live' ? 'bg-red-500/20 text-red-200' : mode === 'paper' ? 'bg-amber-500/20 text-amber-200' : 'bg-blue-500/20 text-blue-200';
