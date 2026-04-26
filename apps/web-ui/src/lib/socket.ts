import { io } from 'socket.io-client';

export const socket = io(process.env.NEXT_PUBLIC_WS_URL ?? 'ws://localhost:4000', {
  autoConnect: false,
  reconnection: true,
  reconnectionDelayMax: 10000,
  transports: ['websocket']
});
