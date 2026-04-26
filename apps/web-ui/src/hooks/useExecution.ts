import { useQuery } from '@tanstack/react-query';
import { api } from '@/lib/api';

export const useExecution = () =>
  useQuery({ queryKey: ['execution'], queryFn: async () => (await api.get('/execution')).data });
