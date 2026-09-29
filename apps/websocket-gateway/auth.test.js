'use strict';

/**
 * Tests for websocket-gateway auth helpers.
 * Run: npm test   (node --test)
 */

const { describe, it, beforeEach, afterEach } = require('node:test');
const assert = require('node:assert/strict');
const jwt = require('jsonwebtoken');

const {
  resolveJwtSecret,
  verifyToken,
  parseAllowedOrigins,
  isOriginAllowed,
  isWsOriginAllowed,
  corsMiddleware,
} = require('./auth');

const TEST_SECRET = 'node-test-secret-that-is-long-enough-0123456789';

function withEnv(vars, fn) {
  const saved = {};
  for (const k of Object.keys(vars)) {
    saved[k] = process.env[k];
    if (vars[k] === undefined) delete process.env[k];
    else process.env[k] = vars[k];
  }
  try {
    fn();
  } finally {
    for (const k of Object.keys(vars)) {
      if (saved[k] === undefined) delete process.env[k];
      else process.env[k] = saved[k];
    }
  }
}

describe('resolveJwtSecret', () => {
  it('rejects missing secret', () => {
    withEnv({ JWT_SECRET: undefined }, () => {
      assert.throws(() => resolveJwtSecret(), /not set/);
    });
  });

  it('rejects denylisted defaults', () => {
    for (const bad of ['change-me', 'change-me-before-production', 'dev-secret', '  change-me  ']) {
      withEnv({ JWT_SECRET: bad }, () => {
        assert.throws(() => resolveJwtSecret(), /forbidden default/, `should reject ${JSON.stringify(bad)}`);
      });
    }
  });

  it('rejects short secrets', () => {
    withEnv({ JWT_SECRET: 'too-short' }, () => {
      assert.throws(() => resolveJwtSecret(), /at least 32/);
    });
  });

  it('accepts a strong secret', () => {
    withEnv({ JWT_SECRET: TEST_SECRET }, () => {
      assert.equal(resolveJwtSecret(), TEST_SECRET);
    });
  });
});

describe('verifyToken', () => {
  function gatewayToken(payload, opts) {
    return jwt.sign(
      { sub: 'op1', role: 'operator', ...payload },
      TEST_SECRET,
      { algorithm: 'HS256', expiresIn: '1h', audience: 'ws-gateway', ...opts },
    );
  }

  it('accepts a valid gateway token', () => {
    const claims = verifyToken(TEST_SECRET, gatewayToken(), 'ws-gateway');
    assert.ok(claims);
    assert.equal(claims.sub, 'op1');
  });

  it('rejects the old dev-token bypass value', () => {
    // The literal 'dev-token' must never authenticate, regardless of env.
    withEnv({ ALLOW_DEV_TOKEN: 'true' }, () => {
      assert.equal(verifyToken(TEST_SECRET, 'dev-token', 'ws-gateway'), null);
    });
  });

  it('rejects missing, malformed, wrong-secret, and expired tokens', () => {
    assert.equal(verifyToken(TEST_SECRET, null, 'ws-gateway'), null);
    assert.equal(verifyToken(TEST_SECRET, '', 'ws-gateway'), null);
    assert.equal(verifyToken(TEST_SECRET, 'garbage', 'ws-gateway'), null);
    const wrongSecret = jwt.sign({ sub: 'op1' }, 'a-different-long-enough-secret-0123456789', { expiresIn: '1h', audience: 'ws-gateway' });
    assert.equal(verifyToken(TEST_SECRET, wrongSecret, 'ws-gateway'), null);
    const expired = jwt.sign({ sub: 'op1' }, TEST_SECRET, { expiresIn: '-1h', audience: 'ws-gateway' });
    assert.equal(verifyToken(TEST_SECRET, expired, 'ws-gateway'), null);
  });

  it('rejects tokens without the gateway audience', () => {
    // No aud at all.
    const noAud = jwt.sign({ sub: 'op1' }, TEST_SECRET, { expiresIn: '1h' });
    assert.equal(verifyToken(TEST_SECRET, noAud, 'ws-gateway'), null);
    // A risk-service token must not authenticate a gateway session, even
    // though both services share JWT_SECRET (cross-service reuse).
    const riskTok = jwt.sign({ sub: 'admin1', role: 'admin' }, TEST_SECRET, { expiresIn: '1h', audience: 'risk-service' });
    assert.equal(verifyToken(TEST_SECRET, riskTok, 'ws-gateway'), null);
  });

  it('rejects tokens when audience is not pinned by the caller', () => {
    const tok = gatewayToken();
    assert.equal(verifyToken(TEST_SECRET, tok, undefined), null);
    assert.equal(verifyToken(TEST_SECRET, tok, ''), null);
  });

  it('rejects non-HS256 algorithms (alg pinning)', () => {
    // Craft a token whose header claims RS256 but is HMAC-signed with the
    // shared secret: the pinned algorithms list must reject it.
    const parts = gatewayToken().split('.');
    const rsHeader = Buffer.from(JSON.stringify({ alg: 'RS256', typ: 'JWT' })).toString('base64url');
    const forged = `${rsHeader}.${parts[1]}.${parts[2]}`;
    assert.equal(verifyToken(TEST_SECRET, forged, 'ws-gateway'), null);
    const noneHeader = Buffer.from(JSON.stringify({ alg: 'none', typ: 'JWT' })).toString('base64url');
    assert.equal(verifyToken(TEST_SECRET, `${noneHeader}.${parts[1]}.`, 'ws-gateway'), null);
  });
});

describe('CORS', () => {
  it('parses ALLOWED_ORIGINS', () => {
    withEnv({ ALLOWED_ORIGINS: 'https://app.example.com, http://localhost:3000 ' }, () => {
      assert.deepEqual(parseAllowedOrigins(), ['https://app.example.com', 'http://localhost:3000']);
    });
    withEnv({ ALLOWED_ORIGINS: undefined }, () => {
      assert.deepEqual(parseAllowedOrigins(), []);
    });
  });

  it('allows only exact configured origins', () => {
    const allowed = ['https://app.example.com'];
    assert.equal(isOriginAllowed('https://app.example.com', allowed), true);
    assert.equal(isOriginAllowed('https://evil.com', allowed), false);
    assert.equal(isOriginAllowed('https://app.example.com.evil.com', allowed), false);
    assert.equal(isOriginAllowed(undefined, allowed), false);
  });

  function mockReqRes(method, origin) {
    const headers = {};
    const res = {
      statusCode: 200,
      ended: false,
      jsonBody: null,
      setHeader(k, v) { headers[k] = v; },
      status(c) { this.statusCode = c; return this; },
      end() { this.ended = true; },
      json(b) { this.jsonBody = b; this.ended = true; },
    };
    return { req: { method, headers: { origin } }, res, headers };
  }

  it('emits no CORS headers when ALLOWED_ORIGINS is unset (fail closed)', () => {
    withEnv({ ALLOWED_ORIGINS: undefined }, () => {
      const mw = corsMiddleware();
      const { req, res, headers } = mockReqRes('GET', 'https://anything.example');
      let nexted = false;
      mw(req, res, () => { nexted = true; });
      assert.equal(nexted, true);
      assert.ok(!('Access-Control-Allow-Origin' in headers), 'must not emit CORS headers');
    });
  });

  it('reflects only configured origins', () => {
    withEnv({ ALLOWED_ORIGINS: 'https://app.example.com' }, () => {
      const mw = corsMiddleware();
      const good = mockReqRes('GET', 'https://app.example.com');
      mw(good.req, good.res, () => {});
      assert.equal(good.headers['Access-Control-Allow-Origin'], 'https://app.example.com');

      const bad = mockReqRes('GET', 'https://evil.com');
      let nexted = false;
      mw(bad.req, bad.res, () => { nexted = true; });
      assert.equal(nexted, true);
      assert.ok(!('Access-Control-Allow-Origin' in bad.headers));
    });
  });

  it('rejects preflight from disallowed origins', () => {
    withEnv({ ALLOWED_ORIGINS: 'https://app.example.com' }, () => {
      const mw = corsMiddleware();
      const { req, res } = mockReqRes('OPTIONS', 'https://evil.com');
      let nexted = false;
      mw(req, res, () => { nexted = true; });
      assert.equal(nexted, false);
      assert.equal(res.statusCode, 403);
    });
  });

  it('enforces WS upgrade origins: absent allowed, unlisted rejected', () => {
    const allowed = ['https://app.example.com'];
    assert.equal(isWsOriginAllowed(undefined, allowed), true);
    assert.equal(isWsOriginAllowed('https://app.example.com', allowed), true);
    assert.equal(isWsOriginAllowed('https://evil.com', allowed), false);
    assert.equal(isWsOriginAllowed('https://app.example.com.evil.com', allowed), false);
  });
});
