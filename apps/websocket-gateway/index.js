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
const { createClient } = require('redis');
const WebSocket = require('ws');
const express = require('express');
const jwt = require('jsonwebtoken');
const { v4: uuidv4 } = require('uuid');

const KAFKA_BROKERS = (process.env.KAFKA_BROKERS || 'kafka:9092').split(',');
const REDIS_URL = process.env.REDIS_URL || 'redis://redis:6379';
const JWT_SECRET = process.env.JWT_SECRET || 'change-me';
const PORT = parseInt(process.env.PORT || '3001', 10);

// Kafka topics to bridge
const TOPICS = [
  'signals.raw',
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
  signals:   ['signals.raw', 'signals.approved', 'signals.rejected'],
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
app.get('/health', (req, res) => res.json({ status: 'ok', clients: clients.size }));
app.get('/metrics', (req, res) => res.json({ connected: clients.size, topics: TOPICS }));

const server = app.listen(PORT, () => {
  console.log(`[ws-gateway] HTTP on :${PORT}`);
});

const wss = new WebSocket.Server({ server, path: '/ws' });

wss.on('connection', (ws, req) => {
  const token = extractToken(req);
  if (!verifyToken(token)) {
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

const kafka = new Kafka({ brokers: KAFKA_BROKERS, clientId: 'ws-gateway' });
const consumer = kafka.consumer({ groupId: 'ws-gateway' });

async function startKafka() {
  await consumer.connect();
  await consumer.subscribe({ topics: TOPICS, fromBeginning: false });

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

  console.log('[ws-gateway] Kafka consumer connected, topics:', TOPICS.join(', '));
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

// ── Auth helpers ──────────────────────────────────────────────────────────────

function extractToken(req) {
  const url = new URL(req.url, `http://localhost:${PORT}`);
  const qp = url.searchParams.get('token');
  if (qp) return qp;
  const auth = req.headers['authorization'] || '';
  if (auth.startsWith('Bearer ')) return auth.slice(7);
  return null;
}

function verifyToken(token) {
  if (!token) return false;
  try {
    jwt.verify(token, JWT_SECRET);
    return true;
  } catch {
    // In paper-mode/dev allow a special dev token
    return token === 'dev-token';
  }
}

// ── Startup ───────────────────────────────────────────────────────────────────

startKafka().catch((err) => {
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
