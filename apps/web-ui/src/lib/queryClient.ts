import { QueryClient } from '@tanstack/react-query';

export const queryClientFactory = () =>
  new QueryClient({
    defaultOptions: {
      queries: { staleTime: 5_000, refetchOnWindowFocus: false }
    }
  });
