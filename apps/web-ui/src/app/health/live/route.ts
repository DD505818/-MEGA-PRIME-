import { NextResponse } from 'next/server';

export const dynamic = 'force-dynamic';

export function GET() {
  return NextResponse.json(
    { status: 'live' },
    { headers: { 'Cache-Control': 'no-store' } }
  );
}
