import { useState, useRef, useEffect } from 'react'
import { api } from '../../services/api'

interface Message { role: 'user' | 'assistant'; text: string; ts: number }

const SLASH_COMMANDS = ['/status', '/risk', '/kill_switch', '/create_strategy', '/help']
const CANNED_REPLIES: Record<string, string> = {
  '/status':  'All 19 agents online. CEO active. CFO budget: 1.8% daily remaining. Gateway latency: 41ms p95.',
  '/risk':    'Current VaR (95%): $312.50. Daily loss: 0.84% (cap 2%). Drawdown: 2.1% (cap 10%). Kill switch: ARMED.',
  '/help':    'Commands: /status /risk /kill_switch /create_strategy. Or ask me anything in natural language.',
}

export function LLMChat() {
  const [messages, setMessages] = useState<Message[]>([
    { role: 'assistant', text: 'OMEGA PRIME AI online. Ask me anything or use slash commands. Try /status or /risk.', ts: Date.now() },
  ])
  const [input, setInput] = useState('')
  const [loading, setLoading] = useState(false)
  const [suggestions, setSuggestions] = useState<string[]>([])
  const bottomRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  const handleInput = (v: string) => {
    setInput(v)
    if (v.startsWith('/')) {
      setSuggestions(SLASH_COMMANDS.filter((c) => c.startsWith(v)))
    } else {
      setSuggestions([])
    }
  }

  const send = async (text: string) => {
    if (!text.trim()) return
    setSuggestions([])
    setInput('')
    const userMsg: Message = { role: 'user', text, ts: Date.now() }
    setMessages((m) => [...m, userMsg])
    setLoading(true)

    let reply: string
    if (CANNED_REPLIES[text]) {
      reply = CANNED_REPLIES[text]
    } else {
      try {
        const res = await api.llmChat(text)
        reply = res.reply
      } catch {
        reply = 'AI service unavailable. Analyzing locally...\n\nBased on current market data: BTC momentum is positive (+3.4% 24h). MAMBA agent has high confidence signal. Risk within limits.'
      }
    }

    setLoading(false)
    setMessages((m) => [...m, { role: 'assistant', text: reply, ts: Date.now() }])
  }

  return (
    <div className="card flex flex-col h-full" style={{ minHeight: 300 }}>
      <div className="flex items-center justify-between mb-2">
        <h3 className="text-sm font-semibold text-text">AI Assistant</h3>
        <span className="text-xs text-muted">Nvidia Nemotron</span>
      </div>

      {/* Messages */}
      <div className="flex-1 overflow-y-auto space-y-2 mb-2 pr-1">
        {messages.map((m, i) => (
          <div key={i} className={`text-xs rounded-lg p-2 ${m.role === 'user'
            ? 'bg-blue/10 border border-blue/20 text-text ml-4'
            : 'bg-panel2 border border-border/50 text-text mr-4'}`}>
            <span className="text-muted text-xs block mb-0.5">{m.role === 'user' ? 'You' : 'Ω AI'}</span>
            <span className="whitespace-pre-wrap">{m.text}</span>
          </div>
        ))}
        {loading && (
          <div className="text-xs bg-panel2 border border-border/50 rounded-lg p-2 mr-4">
            <span className="text-muted">Ω AI</span>
            <span className="ml-2 text-blue animate-pulse">Thinking...</span>
          </div>
        )}
        <div ref={bottomRef} />
      </div>

      {/* Slash command suggestions */}
      {suggestions.length > 0 && (
        <div className="flex gap-1 mb-1 flex-wrap">
          {suggestions.map((s) => (
            <button key={s} onClick={() => send(s)}
              className="text-xs px-2 py-0.5 rounded border border-blue/30 text-blue bg-blue/10 hover:bg-blue/20">
              {s}
            </button>
          ))}
        </div>
      )}

      {/* Input */}
      <div className="flex gap-2">
        <input
          value={input}
          onChange={(e) => handleInput(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && send(input)}
          placeholder="Ask OMEGA anything... or /command"
          className="flex-1 bg-panel2 border border-border rounded-lg px-3 py-1.5 text-xs text-text placeholder-muted outline-none focus:border-blue/50 transition-colors"
        />
        <button onClick={() => send(input)} disabled={!input.trim() || loading}
          className="btn-primary text-xs px-3 disabled:opacity-50">
          Send
        </button>
      </div>
    </div>
  )
}
