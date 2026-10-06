import { proxyUpstream } from '@/lib/upstream';

export const dynamic = 'force-dynamic';

/**
 * GET /api/markets — proxies MARKETS_SERVICE_URL when configured;
 * HTTP 503 { status:'Unavailable' } otherwise. Never fabricates market data.
 */
export async function GET() {
  return proxyUpstream('MARKETS_SERVICE_URL', 'markets');
}
