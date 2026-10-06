import { proxyUpstream } from '@/lib/upstream';

export const dynamic = 'force-dynamic';

/**
 * GET /api/agents — proxies AGENT_SERVICE_URL when configured;
 * HTTP 503 { status:'Unavailable' } otherwise. Never fabricates agent state.
 */
export async function GET() {
  return proxyUpstream('AGENT_SERVICE_URL', 'agents');
}
