import { NextResponse } from 'next/server';
import { LIVE_LOCKED_WORDING } from '@/lib/doctrine';

export const dynamic = 'force-dynamic';

/**
 * GET /api/status — program state for the operator dashboard.
 *
 * Mode contract (mirrors the merged modelock contract): only
 * PAPER_MODE=true counts as paper; anything else — including malformed or
 * absent values — reports UNKNOWN (fail-closed). Live is locked: this
 * interface can never report a live mode.
 */
export async function GET() {
  const mode = process.env.PAPER_MODE === 'true' ? 'PAPER' : 'UNKNOWN';
  return NextResponse.json({
    mode,
    liveLocked: true,
    liveWording: LIVE_LOCKED_WORDING,
    edgeProven: false
  });
}
