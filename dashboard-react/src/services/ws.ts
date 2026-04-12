/**
 * WebSocket client with exponential backoff reconnection.
 * Connects to the gateway-api /ws endpoint.
 */

type MessageHandler = (data: Record<string, unknown>) => void

const WS_URL = (import.meta.env.VITE_WS_URL as string) || 'ws://localhost:8080/ws'
const BACKOFF = [1000, 2000, 4000, 8000, 8000]

class OmegaWebSocket {
  private ws: WebSocket | null = null
  private handlers: Set<MessageHandler> = new Set()
  private attempt = 0
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null
  private running = false

  connect(): void {
    this.running = true
    this._open()
  }

  disconnect(): void {
    this.running = false
    if (this.reconnectTimer) clearTimeout(this.reconnectTimer)
    this.ws?.close()
  }

  subscribe(fn: MessageHandler): () => void {
    this.handlers.add(fn)
    return () => this.handlers.delete(fn)
  }

  send(data: Record<string, unknown>): void {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(data))
    }
  }

  private _open(): void {
    try {
      this.ws = new WebSocket(WS_URL)
      this.ws.onopen    = () => { this.attempt = 0; this._emit({ type: '__connected__' }) }
      this.ws.onclose   = () => { this._emit({ type: '__disconnected__' }); this._schedule() }
      this.ws.onerror   = () => { /* handled by onclose */ }
      this.ws.onmessage = (ev) => {
        try { this._emit(JSON.parse(ev.data) as Record<string, unknown>) }
        catch { /* ignore malformed */ }
      }
    } catch {
      this._schedule()
    }
  }

  private _schedule(): void {
    if (!this.running) return
    const delay = BACKOFF[Math.min(this.attempt, BACKOFF.length - 1)]
    this.attempt++
    this.reconnectTimer = setTimeout(() => this._open(), delay)
  }

  private _emit(data: Record<string, unknown>): void {
    this.handlers.forEach((fn) => fn(data))
  }
}

export const omegaWS = new OmegaWebSocket()
