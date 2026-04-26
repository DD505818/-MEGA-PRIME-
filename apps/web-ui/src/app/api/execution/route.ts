import { NextResponse } from 'next/server';

export async function GET() {
  return NextResponse.json({ data: [], surface: 'execution', ts: new Date().toISOString() });
}
