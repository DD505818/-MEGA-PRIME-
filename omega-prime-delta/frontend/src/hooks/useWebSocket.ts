import { useEffect, useRef, useState } from 'react';

export function useWebSocket(url: string) {
  const [lastMessage, setLastMessage] = useState<string | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectAttempts = useRef(0);
  const maxAttempts = 10;
  const baseDelay = 1000;

  const connect = () => {
    const ws = new WebSocket(url);
    ws.onopen = () => {
      console.log('WebSocket connected');
      reconnectAttempts.current = 0;
    };
    ws.onmessage = (event) => {
      setLastMessage(event.data);
    };
    ws.onclose = () => {
      if (reconnectAttempts.current < maxAttempts) {
        const delay = baseDelay * Math.pow(2, reconnectAttempts.current);
        setTimeout(() => {
          reconnectAttempts.current++;
          connect();
        }, delay);
      }
    };
    ws.onerror = (err) => console.error('WebSocket error', err);
    wsRef.current = ws;
  };

  useEffect(() => {
    connect();
    return () => wsRef.current?.close();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [url]);

  const sendMessage = (data: string) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(data);
    }
  };

  return { lastMessage, sendMessage };
}
