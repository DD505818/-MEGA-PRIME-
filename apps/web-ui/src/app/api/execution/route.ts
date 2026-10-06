import { proxyUpstream } from '@/lib/upstream';

export const dynamic = 'force-dynamic';

/**
 * GET /api/execution — proxies EXECUTION_SERVICE_URL when configured;
 * HTTP 503 { status:'Unavailable' } otherwise. Never fabricates fills or orders.
 */
export async function GET() {
  return proxyUpstream('EXECUTION_SERVICE_URL', 'execution');
}
