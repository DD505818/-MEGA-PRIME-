import { useState, useRef, useEffect, useCallback } from 'react'
import { useLocation } from 'react-router-dom'

interface Message {
  id: string
  role: 'user' | 'assistant'
  content: string
  ts: number
}

const LLM_URL = (import.meta.env.VITE_LLM_URL ?? 'http://localhost:8086')

async function streamChat(
  messages: { role: string; content: string }[],
  onChunk: (text: string) => void,
  onDone: () => void,
  onError: (msg: string) => void,
  signal: AbortSignal,
) {
  const resp = await fetch(`${LLM_URL}/api/v1/llm/chat`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ messages, user_id: 'operator', max_tokens: 1024 }),
    signal,
  })

  if (!resp.ok || !resp.body) {
    onError(`HTTP ${resp.status}`)
    return
  }

  const reader = resp.body.getReader()
  const decoder = new TextDecoder()

  while (true) {
    const { done, value } = await reader.read()
    if (done) break
    const raw = decoder.decode(value, { stream: true })
    for (const line of raw.split('\n')) {
      if (!line.startsWith('data: ')) continue
      const data = line.slice(6).trim()
      if (data === '[DONE]') { onDone(); return }
      if (data.startsWith('[ERROR]')) { onError(data.slice(7).trim()); return }
      try {
        const chunk = JSON.parse(data)
        if (chunk.text) onChunk(chunk.text)
      } catch {
        // ignore malformed SSE lines
      }
    }
  }
  onDone()
}

async function explainTrade(
  trade: Record<string, unknown>,
  onChunk: (text: string) => void,
  onDone: () => void,
  onError: (msg: string) => void,
  signal: AbortSignal,
) {
  const resp = await fetch(`${LLM_URL}/api/v1/llm/explain-trade`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(trade),
    signal,
  })
  if (!resp.ok || !resp.body) { onError(`HTTP ${resp.status}`); return }
  const reader = resp.body.getReader()
  const decoder = new TextDecoder()
  while (true) {
    const { done, value } = await reader.read()
    if (done) break
    const raw = decoder.decode(value, { stream: true })
    for (const line of raw.split('\n')) {
      if (!line.startsWith('data: ')) continue
      const data = line.slice(6).trim()
      if (data === '[DONE]') { onDone(); return }
      if (data.startsWith('[ERROR]')) { onError(data.slice(7).trim()); return }
      try {
        const chunk = JSON.parse(data)
        if (chunk.text) onChunk(chunk.text)
      } catch { /* ignore */ }
    }
  }
  onDone()
}

function MessageBubble({ msg }: { msg: Message }) {
  const isUser = msg.role === 'user'
  return (
    <div className={`flex ${isUser ? 'justify-end' : 'justify-start'} mb-3`}>
      <div
        className={`max-w-[78%] rounded-lg px-3 py-2 text-sm leading-relaxed whitespace-pre-wrap ${
          isUser
            ? 'bg-blue/20 text-text border border-blue/30'
            : 'bg-panel2 text-text border border-border'
        }`}
      >
        {!isUser && (
          <span className="text-[10px] font-bold text-gold tracking-widest block mb-1">
            AI STRATEGIST
          </span>
        )}
        {msg.content}
      </div>
    </div>
  )
}

export function AIStrategistView() {
  const location = useLocation()
  const [messages, setMessages] = useState<Message[]>([])
  const [input, setInput] = useState('')
  const [streaming, setStreaming] = useState(false)
  const [spendInfo, setSpendInfo] = useState<{ spent: number; cap: number; kill: boolean } | null>(null)
  const abortRef = useRef<AbortController | null>(null)
  const bottomRef = useRef<HTMLDivElement>(null)

  // Auto-scroll to bottom on new messages
  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  // Fetch status on mount
  useEffect(() => {
    fetch(`${LLM_URL}/api/v1/llm/status`)
      .then((r) => r.json())
      .then((d) =>
        setSpendInfo({ spent: d.daily_spend_usd, cap: d.daily_cap_usd, kill: d.kill_switch }),
      )
      .catch(() => null)
  }, [])

  // Handle "explain this trade" deep link from dashboard
  useEffect(() => {
    const params = new URLSearchParams(location.search)
    const tradeParam = params.get('trade')
    if (!tradeParam) return
    try {
      const trade = JSON.parse(decodeURIComponent(tradeParam))
      const userMsg: Message = {
        id: crypto.randomUUID(),
        role: 'user',
        content: `Explain this trade:\n\`\`\`json\n${JSON.stringify(trade, null, 2)}\n\`\`\``,
        ts: Date.now(),
      }
      const assistantMsg: Message = { id: crypto.randomUUID(), role: 'assistant', content: '', ts: Date.now() }
      setMessages([userMsg, assistantMsg])
      setStreaming(true)
      const ctrl = new AbortController()
      abortRef.current = ctrl
      explainTrade(
        trade,
        (chunk) =>
          setMessages((prev) =>
            prev.map((m) => (m.id === assistantMsg.id ? { ...m, content: m.content + chunk } : m)),
          ),
        () => setStreaming(false),
        (err) => {
          setMessages((prev) =>
            prev.map((m) =>
              m.id === assistantMsg.id ? { ...m, content: `Error: ${err}` } : m,
            ),
          )
          setStreaming(false)
        },
        ctrl.signal,
      )
    } catch {
      // ignore malformed deep-link
    }
  }, [location.search])

  const sendMessage = useCallback(async () => {
    const text = input.trim()
    if (!text || streaming) return
    setInput('')

    const userMsg: Message = { id: crypto.randomUUID(), role: 'user', content: text, ts: Date.now() }
    const assistantMsg: Message = { id: crypto.randomUUID(), role: 'assistant', content: '', ts: Date.now() }

    setMessages((prev) => [...prev, userMsg, assistantMsg])
    setStreaming(true)

    const history = [...messages, userMsg].map((m) => ({ role: m.role, content: m.content }))

    const ctrl = new AbortController()
    abortRef.current = ctrl

    await streamChat(
      history,
      (chunk) =>
        setMessages((prev) =>
          prev.map((m) => (m.id === assistantMsg.id ? { ...m, content: m.content + chunk } : m)),
        ),
      () => {
        setStreaming(false)
        // Refresh spend info after each call
        fetch(`${LLM_URL}/api/v1/llm/status`)
          .then((r) => r.json())
          .then((d) => setSpendInfo({ spent: d.daily_spend_usd, cap: d.daily_cap_usd, kill: d.kill_switch }))
          .catch(() => null)
      },
      (err) => {
        setMessages((prev) =>
          prev.map((m) =>
            m.id === assistantMsg.id ? { ...m, content: `Error: ${err}` } : m,
          ),
        )
        setStreaming(false)
      },
      ctrl.signal,
    )
  }, [input, streaming, messages])

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      sendMessage()
    }
  }

  const stopStreaming = () => {
    abortRef.current?.abort()
    setStreaming(false)
  }

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-2 border-b border-border bg-panel/80">
        <div>
          <span className="text-gold font-bold tracking-widest text-xs">AI STRATEGIST</span>
          <span className="ml-2 text-muted text-xs">powered by Kimi K2 · research only</span>
        </div>
        {spendInfo && (
          <div className="flex items-center gap-3 text-xs font-mono">
            {spendInfo.kill && (
              <span className="text-red-500 font-bold animate-pulse">KILL SWITCH ACTIVE</span>
            )}
            <span className="text-muted">
              Daily spend:{' '}
              <span className={spendInfo.spent / spendInfo.cap > 0.8 ? 'text-red-400' : 'text-green'}>
                ${spendInfo.spent.toFixed(4)}
              </span>
              {' / '}
              <span className="text-muted">${spendInfo.cap.toFixed(2)}</span>
            </span>
            <div
              className="h-1.5 w-20 rounded-full bg-panel2 overflow-hidden"
              title={`${((spendInfo.spent / spendInfo.cap) * 100).toFixed(1)}% of daily cap`}
            >
              <div
                className={`h-full rounded-full transition-all ${
                  spendInfo.spent / spendInfo.cap > 0.8
                    ? 'bg-red-500'
                    : spendInfo.spent / spendInfo.cap > 0.5
                    ? 'bg-yellow-400'
                    : 'bg-green'
                }`}
                style={{ width: `${Math.min(100, (spendInfo.spent / spendInfo.cap) * 100)}%` }}
              />
            </div>
          </div>
        )}
      </div>

      {/* Disclaimer banner */}
      <div className="px-4 py-1.5 bg-yellow-500/10 border-b border-yellow-500/20 text-yellow-300/80 text-[10px] font-mono">
        RESEARCH ONLY — AI Strategist provides market context and analysis. It does not issue buy/sell
        signals and makes no guarantees about future performance. All trading decisions are made by the
        autonomous agents subject to AEGIS risk gates.
      </div>

      {/* Message list */}
      <div className="flex-1 overflow-y-auto px-4 py-4">
        {messages.length === 0 && (
          <div className="flex flex-col items-center justify-center h-full text-muted">
            <div className="text-4xl mb-3 opacity-20">Ω</div>
            <p className="text-sm mb-1">Ask about market structure, strategy signals, or risk metrics.</p>
            <p className="text-xs opacity-60">
              Examples: "What is the SURGE agent detecting right now?" · "Explain the Kelly criterion."
            </p>
          </div>
        )}
        {messages.map((msg) => (
          <MessageBubble key={msg.id} msg={msg} />
        ))}
        {streaming && messages[messages.length - 1]?.role === 'assistant' && messages[messages.length - 1]?.content === '' && (
          <div className="flex justify-start mb-3">
            <div className="bg-panel2 border border-border rounded-lg px-3 py-2 text-sm">
              <span className="text-gold text-[10px] font-bold tracking-widest block mb-1">AI STRATEGIST</span>
              <span className="inline-block w-2 h-4 bg-gold/80 animate-pulse" />
            </div>
          </div>
        )}
        <div ref={bottomRef} />
      </div>

      {/* Input area */}
      <div className="px-4 py-3 border-t border-border bg-panel/80">
        <div className="flex gap-2">
          <textarea
            className="flex-1 bg-panel2 border border-border rounded text-sm text-text px-3 py-2 resize-none focus:outline-none focus:border-blue/60 font-mono"
            rows={2}
            placeholder="Ask about market context, signals, risk metrics… (Shift+Enter for new line)"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={handleKeyDown}
            disabled={streaming}
          />
          {streaming ? (
            <button
              onClick={stopStreaming}
              className="px-3 py-2 rounded bg-red-500/20 border border-red-500/40 text-red-400 text-xs font-bold hover:bg-red-500/30 transition-colors"
            >
              STOP
            </button>
          ) : (
            <button
              onClick={sendMessage}
              disabled={!input.trim()}
              className="px-4 py-2 rounded bg-blue/20 border border-blue/40 text-blue text-xs font-bold hover:bg-blue/30 transition-colors disabled:opacity-30 disabled:cursor-not-allowed"
            >
              SEND
            </button>
          )}
        </div>
      </div>
    </div>
  )
}
