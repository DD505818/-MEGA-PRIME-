import { NextResponse } from 'next/server';

export async function GET() {
  return NextResponse.json({ data: [], surface: 'markets', ts: new Date().toISOString() });
}
