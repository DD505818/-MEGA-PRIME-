import { useQuery } from '@tanstack/react-query';
import { api } from '@/lib/api';

export interface StatusResponse {
  mode: 'PAPER' | 'UNKNOWN';
  liveLocked: boolean;
  liveWording: string;
  edgeProven: boolean;
}

/**
 * Program status from /api/status — the only source of mode truth for the UI.
 * Anything other than an explicit PAPER response renders as UNKNOWN.
 */
export const useStatus = () =>
  useQuery({
    queryKey: ['status'],
    queryFn: async () => (await api.get('/status')).data as StatusResponse
  });
