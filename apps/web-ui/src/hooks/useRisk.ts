import { useQuery } from '@tanstack/react-query';
import { api } from '@/lib/api';

export const useRisk = () => useQuery({ queryKey: ['risk'], queryFn: async () => (await api.get('/risk')).data });
