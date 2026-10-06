import { NextResponse } from 'next/server';

/**
 * Honest upstream proxy helper for the operator dashboard API routes.
 *
 * GET-only. If the upstream URL env var is unset, or the upstream is
 * unreachable or errors, this returns HTTP 503 with { status:'Unavailable' }
 * — the UI must render UNKNOWN/empty states, never synthetic numbers.
 */
export async function proxyUpstream(envVar: string, surface: string): Promise<NextResponse> {
  const upstream = process.env[envVar];
  if (!upstream) {
    return NextResponse.json(
      { status: 'Unavailable', reason: `${envVar} not configured`, surface },
      { status: 503 }
    );
  }
  try {
    const res = await fetch(upstream, { cache: 'no-store' });
    if (!res.ok) {
      return NextResponse.json(
        { status: 'Unavailable', reason: `upstream responded HTTP ${res.status}`, surface },
        { status: 503 }
      );
    }
    const data = await res.json();
    return NextResponse.json(data);
  } catch {
    return NextResponse.json(
      { status: 'Unavailable', reason: 'upstream unreachable', surface },
      { status: 503 }
    );
  }
}
