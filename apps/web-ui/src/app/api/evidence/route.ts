import { NextResponse } from 'next/server';
import { loadEvidence } from '@/lib/evidence';

export const dynamic = 'force-dynamic';

/**
 * GET /api/evidence — edge-campaign evidence for the operator dashboard.
 *
 * GET only. Reads frozen campaign artifacts from the directory named by
 * $EVIDENCE_DIR (default: /evidence); see src/lib/evidence.ts for the
 * expected mount layout. When the directory or its files are absent this
 * returns HTTP 200 with { available:false, status:'UNKNOWN' } — never
 * fabricated evidence.
 */
export async function GET() {
  const payload = await loadEvidence();
  return NextResponse.json(payload);
}
