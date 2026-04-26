import { NextResponse } from 'next/server';

export async function GET() {
  return NextResponse.json({ data: [], surface: 'portfolio', ts: new Date().toISOString() });
}
