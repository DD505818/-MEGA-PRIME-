import { useQuery } from '@tanstack/react-query';
import { api } from '@/lib/api';

export const usePortfolio = () =>
  useQuery({
    queryKey: ['portfolio'],
    queryFn: async () => (await api.get('/portfolio')).data
  });
