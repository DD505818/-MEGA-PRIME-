const uWS = require('uWebSockets.js');
const { Kafka } = require('kafkajs');

const kafka = new Kafka({ brokers: ['kafka:9092'] });
const consumer = kafka.consumer({ groupId: 'ws-gateway' });
const clients = new Map();

const start = async () => {
  await consumer.connect();
  await consumer.subscribe({ topics: ['market.ticks', 'portfolio.updated', 'risk.alerts', 'agents.metrics'] });
  await consumer.run({
    eachMessage: async ({ topic, message }) => {
      const data = JSON.parse(message.value.toString());
      const payload = JSON.stringify({ topic, data });
      for (const ws of clients.values()) {
        ws.send(payload);
      }
    },
  });
};

uWS.App()
  .ws('/*', {
    open: (ws) => {
      const id = Math.random().toString(36).substr(2, 9);
      clients.set(id, ws);
      ws.send(JSON.stringify({ type: 'connected', id }));
    },
    message: (ws, message, isBinary) => {
      const text = Buffer.from(message).toString();
      void text;
    },
    close: (ws) => {
      for (const [id, client] of clients) if (client === ws) clients.delete(id);
    },
  })
  .listen(3001, (listenSocket) => {
    if (listenSocket) {
      console.log('WebSocket gateway listening on port 3001');
      start();
    }
  });
