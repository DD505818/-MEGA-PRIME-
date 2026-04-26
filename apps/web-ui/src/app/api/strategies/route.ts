import { NextResponse } from 'next/server';

export async function GET() {
  return NextResponse.json({ data: [], surface: 'strategies', ts: new Date().toISOString() });
}
