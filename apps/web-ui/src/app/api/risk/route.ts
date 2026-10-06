import { proxyUpstream } from '@/lib/upstream';

export const dynamic = 'force-dynamic';

/**
 * GET /api/risk — proxies RISK_SERVICE_URL when configured;
 * HTTP 503 { status:'Unavailable' } otherwise. Never fabricates risk state.
 */
export async function GET() {
  return proxyUpstream('RISK_SERVICE_URL', 'risk');
}
