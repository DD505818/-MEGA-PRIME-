import { proxyUpstream } from '@/lib/upstream';

// No dedicated reports service exists in the stack; REPORTS_SERVICE_URL is
// unset by default, so this route honestly returns 503 Unavailable rather
// than an empty stub that could be mistaken for "no reports".
export async function GET() {
  return proxyUpstream('REPORTS_SERVICE_URL', 'reports');
}
