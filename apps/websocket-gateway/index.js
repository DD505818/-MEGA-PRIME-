'use strict';

/**
 * ΩMEGA PRIME Δ — WebSocket Gateway
 *
 * Bridges Kafka topics to authenticated WebSocket clients in real time.
 * Topics consumed: signals.raw, signals.approved, signals.rejected,
 *                  orders.fills, risk.alerts, emergency.halt, market.prices
 *
 * Authentication: JWT Bearer in query param `token` or Authorization header.
 * Each client subscribes to specific channels via { type:'subscribe', channels:[] }.
 */

const { Kafka } = require('kafkajs');
const fs = require('fs');
const { createClient } = require('redis');
const WebSocket = require('ws');
const express = require('express');
const { v4: uuidv4 } = require('uuid');
const {
  resolveJwtSecret,
  verifyToken,
  parseAllowedOrigins,
  isWsOriginAllowed,
  corsMiddleware,
  GATEWAY_AUDIENCE,
} = require('./auth');

const KAFKA_BROKERS = (process.env.KAFKA_BROKERS || 'kafka:9092').split(',');
const KAFKA_PROTOCOL = (process.env.KAFKA_SECURITY_PROTOCOL || 'PLAINTEXT').toUpperCase();

function kafkaTransport() {
  const options = { brokers: KAFKA_BROKERS, clientId: 'ws-gateway' };
  if (KAFKA_PROTOCOL === 'SSL' || KAFKA_PROTOCOL === 'SASL_SSL') {
    const caPath = process.env.KAFKA_SSL_CA_LOCATION;
    options.ssl = caPath ? { ca: [fs.readFileSync(caPath, 'utf8')] } : true;
  }
  if (KAFKA_PROTOCOL === 'SASL_SSL' || KAFKA_PROTOCOL === 'SASL_PLAINTEXT') {
    options.sasl = {
      mechanism: (process.env.KAFKA_SASL_MECHANISM || 'plain').toLowerCase(),
      username: process.env.KAFKA_SASL_USERNAME,
      password: process.env.KAFKA_SASL_PASSWORD,
    };
  }
  return options;
}
const REDIS_URL = process.env.REDIS_URL || 'redis://redis:6379';
// Fail closed: the gateway must not start without a real JWT secret.
// resolveJwtSecret() throws on missing/denylisted/short values.
const JWT_SECRET = resolveJwtSecret();
// WS upgrade origins are pinned at startup from ALLOWED_ORIGINS.
const WS_ALLOWED_ORIGINS = parseAllowedOrigins();
const PORT = parseInt(process.env.PORT || '3001', 10);
let kafkaReady = false;

// Kafka topics to bridge
const TOPICS = [
  'signals.raw',
  'signals.fused',    // 1B.4: fusion-engine consensus output (observe)
  'signals.sized',    // 1B.4: capital-allocator output (observe)
  'signals.approved',
  'signals.rejected',
  'orders.fills',
  'orders.routed',
  'risk.alerts',
  'emergency.halt',
  'market.prices',
  'portfolio.state',
];

// Channel → topic mapping for client subscriptions
const CHANNEL_TOPICS = {
  signals:   ['signals.raw', 'signals.fused', 'signals.sized', 'signals.approved', 'signals.rejected'],
  orders:    ['orders.fills', 'orders.routed'],
  risk:      ['risk.alerts', 'emergency.halt'],
  prices:    ['market.prices'],
  portfolio: ['portfolio.state'],
  all:       TOPICS,
};

// ── State ─────────────────────────────────────────────────────────────────────

/** @type {Map<string, { ws: WebSocket, channels: Set<string>, id: string }>} */
const clients = new Map();

// ── HTTP + WS server ──────────────────────────────────────────────────────────

const app = express();
// CORS is restricted to ALLOWED_ORIGINS. When unset, no CORS headers are
// emitted at all (fail closed) — browsers fall back to same-origin.
app.use(corsMiddleware());
app.get('/health', (req, res) => res.json({ status: 'live', clients: clients.size }));
app.get('/health/live', (req, res) => res.json({ status: 'live' }));
app.get('/health/ready', (req, res) => {
  const redisReady = redisClient?.isReady === true;
  if (!kafkaReady || !redisReady) {
    return res.status(503).json({
      status: 'not_ready',
      dependencies: { kafka: kafkaReady, redis: redisReady },
    });
  }
  return res.json({ status: 'ready' });
});
app.get('/metrics', (req, res) => res.json({ connected: clients.size, topics: TOPICS }));

const server = app.listen(PORT, () => {
  console.log(`[ws-gateway] HTTP on :${PORT}`);
});

const wss = new WebSocket.Server({ server, path: '/ws' });

wss.on('connection', (ws, req) => {
  // Browser-origin enforcement on upgrade: a present but unlisted Origin
  // is rejected. (Absent Origin = non-browser tooling; JWT still required.)
  if (!isWsOriginAllowed(req.headers.origin, WS_ALLOWED_ORIGINS)) {
    ws.close(4003, 'Origin not allowed');
    return;
  }
  const token = extractToken(req);
  // Audience is pinned: only tokens minted for this gateway authenticate.
  const claims = verifyToken(JWT_SECRET, token, GATEWAY_AUDIENCE);
  if (!claims) {
    ws.close(4001, 'Unauthorized');
    return;
  }

  const clientId = uuidv4();
  const client = { ws, channels: new Set(['all']), id: clientId };
  clients.set(clientId, client);

  ws.send(JSON.stringify({ type: 'connected', clientId, ts: Date.now() }));
  console.log(`[ws-gateway] client ${clientId} connected (total=${clients.size})`);

  ws.on('message', (data) => {
    try {
      const msg = JSON.parse(data.toString());
      if (msg.type === 'subscribe' && Array.isArray(msg.channels)) {
        client.channels = new Set(msg.channels);
        ws.send(JSON.stringify({ type: 'subscribed', channels: msg.channels }));
      }
      if (msg.type === 'ping') {
        ws.send(JSON.stringify({ type: 'pong', ts: Date.now() }));
      }
    } catch {}
  });

  ws.on('close', () => {
    clients.delete(clientId);
    console.log(`[ws-gateway] client ${clientId} disconnected (total=${clients.size})`);
  });

  ws.on('error', (err) => {
    console.error(`[ws-gateway] client ${clientId} error:`, err.message);
    clients.delete(clientId);
  });
});

// ── Kafka consumer ────────────────────────────────────────────────────────────

const kafka = new Kafka(kafkaTransport());
const consumer = kafka.consumer({ groupId: 'ws-gateway' });

async function startKafka() {
  await consumer.connect();
  await consumer.subscribe({ topics: TOPICS, fromBeginning: false });
  kafkaReady = true;
  console.log('[ws-gateway] Kafka consumer connected, topics:', TOPICS.join(', '));

  await consumer.run({
    eachMessage: async ({ topic, message }) => {
      let payload;
      try {
        payload = JSON.parse(message.value.toString());
      } catch {
        payload = { raw: message.value.toString() };
      }

      const envelope = JSON.stringify({
        type: 'event',
        topic,
        ts: Date.now(),
        data: payload,
      });

      broadcast(topic, envelope);
    },
  });

}

// ── Redis health publisher ────────────────────────────────────────────────────

const redisClient = createClient({ url: REDIS_URL });
redisClient.connect().catch((err) =>
  console.warn('[ws-gateway] Redis connect error:', err.message)
);

setInterval(async () => {
  try {
    await redisClient.set('gateway:heartbeat', Date.now(), { EX: 10 });
  } catch {}
}, 5000);

// ── Broadcast ─────────────────────────────────────────────────────────────────

function broadcast(topic, envelope) {
  if (clients.size === 0) return;

  for (const [, client] of clients) {
    if (!wantsChannel(client.channels, topic)) continue;
    if (client.ws.readyState !== WebSocket.OPEN) continue;
    try {
      client.ws.send(envelope);
    } catch {}
  }
}

function wantsChannel(subscribed, topic) {
  if (subscribed.has('all')) return true;
  for (const ch of subscribed) {
    const mapped = CHANNEL_TOPICS[ch] || [];
    if (mapped.includes(topic)) return true;
  }
  return false;
}

// ── Token extraction ────────────────────────────────────────────────────────

function extractToken(req) {
  const url = new URL(req.url, `http://localhost:${PORT}`);
  const qp = url.searchParams.get('token');
  if (qp) return qp;
  const auth = req.headers['authorization'] || '';
  if (auth.startsWith('Bearer ')) return auth.slice(7);
  return null;
}

// NOTE: token verification lives in auth.js (verifyToken). The dev-token
// bypass was removed: there is no way to connect without a valid JWT.

// ── Startup ───────────────────────────────────────────────────────────────────

startKafka().catch((err) => {
  kafkaReady = false;
  console.error('[ws-gateway] Kafka startup error:', err.message);
  // Retry after 5s
  setTimeout(() => startKafka(), 5000);
});

process.on('SIGINT', async () => {
  console.log('[ws-gateway] Shutting down...');
  await consumer.disconnect();
  server.close();
  process.exit(0);
});
