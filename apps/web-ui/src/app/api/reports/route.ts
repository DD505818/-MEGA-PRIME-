import { NextResponse } from 'next/server';

export async function GET() {
  return NextResponse.json({ data: [], surface: 'reports', ts: new Date().toISOString() });
}
