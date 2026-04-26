import { NextResponse } from 'next/server';
import type { NextRequest } from 'next/server';

const protectedRoutes = ['/', '/markets', '/portfolio', '/agents', '/studio', '/rl', '/execution', '/risk', '/reports', '/settings'];

export function middleware(req: NextRequest) {
  if (protectedRoutes.includes(req.nextUrl.pathname)) {
    const authorized = req.cookies.get('next-auth.session-token') || req.cookies.get('__Secure-next-auth.session-token');
    if (!authorized) {
      return NextResponse.redirect(new URL('/login', req.url));
    }
  }

  return NextResponse.next();
}

export const config = { matcher: ['/', '/markets', '/portfolio', '/agents', '/studio', '/rl', '/execution', '/risk', '/reports', '/settings'] };
