import { useCallback, useEffect, useRef, useState } from 'react';

export function useWebSocket(url: string) {
  const [lastMessage, setLastMessage] = useState<string | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectAttempts = useRef(0);
  const maxAttempts = 10;
  const baseDelay = 1000;

  const connect = useCallback(() => {
    const ws = new WebSocket(url);
    ws.onopen = () => {
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
    ws.onerror = () => {
      ws.close();
    };
    wsRef.current = ws;
  }, [url]);

  useEffect(() => {
    connect();
    return () => wsRef.current?.close();
  }, [connect]);

  const sendMessage = (data: string) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(data);
    }
  };

  return { lastMessage, sendMessage };
}
