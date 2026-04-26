import { NextResponse } from 'next/server';

export async function GET() {
  return NextResponse.json({ data: [], surface: 'risk', ts: new Date().toISOString() });
}
