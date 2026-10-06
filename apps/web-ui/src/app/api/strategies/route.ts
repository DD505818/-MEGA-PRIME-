import { proxyUpstream } from '@/lib/upstream';

// Strategy/MIDAS registry reads belong to the agent-service, which exposes no
// HTTP registry endpoint in this stack; STRATEGIES_SERVICE_URL is unset by
// default, so this route honestly returns 503 Unavailable.
export async function GET() {
  return proxyUpstream('STRATEGIES_SERVICE_URL', 'strategies');
}
