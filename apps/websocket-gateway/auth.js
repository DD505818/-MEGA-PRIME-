'use strict';

/**
 * WebSocket gateway authentication helpers.
 *
 * Fail-closed: the process must not start unless JWT_SECRET is set to a
 * non-default value of at least 32 characters. There is no dev-token bypass
 * and no hardcoded secret fallback.
 */

const jwt = require('jsonwebtoken');

const SECRET_DENYLIST = new Set([  '',
  'change-me',
  'change-me-before-production',
  'dev-secret',
  'secret',
  'password',
]);

const MIN_SECRET_LEN = 32;

// Audience this gateway requires in bearer tokens. A token minted for the
// risk-service (aud "risk-service") is rejected here, blocking cross-service
// token reuse even though both services share JWT_SECRET.
const GATEWAY_AUDIENCE = 'ws-gateway';

/**
 * Resolve the JWT secret, throwing if it is absent, denylisted, or too short.
 * Callers should let this throw at startup (fail fast).
 */
function resolveJwtSecret() {
  const s = (process.env.JWT_SECRET || '').trim();
  if (s === '') {
    throw new Error('JWT_SECRET is not set — refusing to start without gateway auth (see .env.example)');
  }
  if (SECRET_DENYLIST.has(s)) {
    throw new Error('JWT_SECRET uses a forbidden default value — generate a fresh secret (see .env.example)');
  }
  if (s.length < MIN_SECRET_LEN) {
    throw new Error(`JWT_SECRET must be at least ${MIN_SECRET_LEN} characters (got ${s.length})`);
  }
  return s;
}

/**
 * Verify a bearer token. Returns the decoded payload on success, null on any
 * failure (missing, malformed, bad signature, expired, wrong audience).
 *
 * The algorithm is pinned to HS256 explicitly — the header's alg value is
 * never trusted to select a verification method, so alg-confusion attacks
 * (e.g. an RS256 header) cannot route into another code path.
 * The audience is required: a risk-service token must not authenticate a
 * gateway session and vice versa, even though both share JWT_SECRET.
 */
function verifyToken(secret, token, audience) {
  if (!token || typeof token !== 'string') return null;
  if (!audience || typeof audience !== 'string') return null;
  try {
    return jwt.verify(token, secret, { algorithms: ['HS256'], audience });
  } catch {
    return null;
  }
}

/**
 * Parse ALLOWED_ORIGINS (comma-separated) into a list of exact origins.
 * Empty list = no cross-origin access (most restrictive default).
 */
function parseAllowedOrigins() {
  return (process.env.ALLOWED_ORIGINS || '')
    .split(',')
    .map((o) => o.trim())
    .filter(Boolean);
}

function isOriginAllowed(origin, allowedOrigins) {
  if (!origin) return false;
  return allowedOrigins.includes(origin);
}

/**
 * Origin policy for WebSocket upgrades. Browser clients send an Origin
 * header; non-browser operator tooling typically does not. A present but
 * unlisted origin is rejected (4003); an absent origin is allowed because
 * authentication is still enforced via JWT on every connection.
 */
function isWsOriginAllowed(origin, allowedOrigins) {
  if (!origin) return true;
  return allowedOrigins.includes(origin);
}

/**
 * Express middleware restricting CORS to configured origins only.
 * When ALLOWED_ORIGINS is unset, no CORS headers are emitted (fail closed).
 */
function corsMiddleware() {
  const allowed = parseAllowedOrigins();
  return (req, res, next) => {
    const origin = req.headers.origin;
    if (origin && isOriginAllowed(origin, allowed)) {
      res.setHeader('Access-Control-Allow-Origin', origin);
      res.setHeader('Vary', 'Origin');
      res.setHeader('Access-Control-Allow-Methods', 'GET,POST,OPTIONS');
      res.setHeader('Access-Control-Allow-Headers', 'Authorization,Content-Type');
      if (req.method === 'OPTIONS') {
        res.status(204).end();
        return;
      }
    } else if (req.method === 'OPTIONS') {
      // Preflight from a disallowed origin: explicit rejection.
      res.status(403).json({ error: 'origin not allowed' });
      return;
    }
    next();
  };
}

module.exports = {
  resolveJwtSecret,
  verifyToken,
  parseAllowedOrigins,
  isOriginAllowed,
  isWsOriginAllowed,
  corsMiddleware,
  GATEWAY_AUDIENCE,
  SECRET_DENYLIST,
  MIN_SECRET_LEN,
};
