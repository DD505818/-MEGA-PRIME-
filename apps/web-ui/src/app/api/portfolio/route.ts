import { proxyUpstream } from '@/lib/upstream';

export const dynamic = 'force-dynamic';

/**
 * GET /api/portfolio — proxies PORTFOLIO_SERVICE_URL when configured;
 * HTTP 503 { status:'Unavailable' } otherwise. Never fabricates balances or P&L.
 */
export async function GET() {
  return proxyUpstream('PORTFOLIO_SERVICE_URL', 'portfolio');
}
