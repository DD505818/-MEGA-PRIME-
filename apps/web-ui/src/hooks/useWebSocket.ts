'use client';

import { useEffect, useState } from 'react';
import { socket } from '@/lib/socket';
import { STREAM_EVENTS } from '@/lib/constants';

export function useWebSocket() {
  const [connected, setConnected] = useState(false);

  useEffect(() => {
    socket.connect();
    const onConnect = () => setConnected(true);
    const onDisconnect = () => setConnected(false);

    socket.on('connect', onConnect);
    socket.on('disconnect', onDisconnect);
    STREAM_EVENTS.forEach((event) => socket.on(event, () => undefined));

    return () => {
      socket.off('connect', onConnect);
      socket.off('disconnect', onDisconnect);
      STREAM_EVENTS.forEach((event) => socket.off(event));
      socket.disconnect();
    };
  }, []);

  return { connected };
}
