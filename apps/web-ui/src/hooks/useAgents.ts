import { useQuery } from '@tanstack/react-query';
import { api } from '@/lib/api';

export const useAgents = () =>
  useQuery({ queryKey: ['agents'], queryFn: async () => (await api.get('/agents')).data });
