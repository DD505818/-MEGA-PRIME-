import { useQuery } from '@tanstack/react-query';
import { api } from '@/lib/api';

export const useMarketData = () =>
  useQuery({ queryKey: ['markets'], queryFn: async () => (await api.get('/markets')).data });
