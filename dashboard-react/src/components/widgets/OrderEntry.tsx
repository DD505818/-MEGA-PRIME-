import { useState } from 'react'
import { useAppStore } from '../../store/useAppStore'
import clsx from 'clsx'

const SYMBOLS = ['BTC/USD', 'ETH/USD', 'SOL/USD', 'AVAX/USD']
const ORDER_TYPES = ['MARKET', 'LIMIT', 'TWAP', 'ICEBERG']

export function OrderEntry() {
  const ticks = useAppStore((s) => s.ticks)
  const [symbol, setSymbol] = useState('BTC/USD')
  const [side, setSide] = useState<'BUY' | 'SELL'>('BUY')
  const [orderType, setOrderType] = useState('MARKET')
  const [qty, setQty] = useState('')
  const [price, setPrice] = useState('')
  const [submitted, setSubmitted] = useState(false)

  const tick = ticks[symbol]
  const currentPrice = tick?.price ?? 0
  const notional = currentPrice * parseFloat(qty || '0')
  const slippage = orderType === 'MARKET' ? currentPrice * 0.0002 : 0
  const maxLoss = notional * 0.005

  const submit = () => {
    if (!qty) return
    setSubmitted(true)
    setTimeout(() => setSubmitted(false), 2000)
  }

  return (
    <div className="card flex flex-col gap-3">
      <h3 className="text-sm font-semibold text-text">Order Entry</h3>

      {/* Symbol */}
      <select value={symbol} onChange={(e) => setSymbol(e.target.value)}
        className="bg-panel2 border border-border rounded-lg px-2 py-1.5 text-sm text-text outline-none focus:border-blue/50">
        {SYMBOLS.map((s) => <option key={s}>{s}</option>)}
      </select>

      {/* Side */}
      <div className="grid grid-cols-2 gap-2">
        <button onClick={() => setSide('BUY')}
          className={clsx('py-2 rounded-lg text-sm font-bold border transition-all',
            side === 'BUY' ? 'bg-green/20 border-green/60 text-green' : 'border-border text-muted hover:border-green/30')}>
          BUY
        </button>
        <button onClick={() => setSide('SELL')}
          className={clsx('py-2 rounded-lg text-sm font-bold border transition-all',
            side === 'SELL' ? 'bg-red/20 border-red/60 text-red' : 'border-border text-muted hover:border-red/30')}>
          SELL
        </button>
      </div>

      {/* Order type */}
      <div className="flex gap-1 flex-wrap">
        {ORDER_TYPES.map((t) => (
          <button key={t} onClick={() => setOrderType(t)}
            className={clsx('px-2 py-1 rounded text-xs border transition-all',
              t === orderType ? 'bg-panel2 border-border text-text' : 'border-transparent text-muted hover:text-text')}>
            {t}
          </button>
        ))}
      </div>

      {/* Quantity */}
      <div>
        <p className="kicker">Quantity</p>
        <input value={qty} onChange={(e) => setQty(e.target.value)} type="number" min="0"
          placeholder="0.00"
          className="w-full bg-panel2 border border-border rounded-lg px-3 py-2 text-sm font-mono text-text outline-none focus:border-blue/50" />
        <div className="flex gap-1 mt-1">
          {['10%', '25%', '50%', '100%'].map((p) => (
            <button key={p} className="flex-1 text-xs text-muted border border-border/50 rounded py-0.5 hover:text-text hover:border-border transition-all">
              {p}
            </button>
          ))}
        </div>
      </div>

      {/* Price (limit only) */}
      {orderType !== 'MARKET' && (
        <div>
          <p className="kicker">Limit Price</p>
          <input value={price || currentPrice.toFixed(2)} onChange={(e) => setPrice(e.target.value)} type="number"
            className="w-full bg-panel2 border border-border rounded-lg px-3 py-2 text-sm font-mono text-text outline-none focus:border-blue/50" />
        </div>
      )}

      {/* Preview */}
      {qty && parseFloat(qty) > 0 && (
        <div className="border border-border/50 rounded-lg p-2 space-y-1 text-xs font-mono">
          <div className="flex justify-between"><span className="text-muted">Notional</span><span>${notional.toFixed(2)}</span></div>
          <div className="flex justify-between"><span className="text-muted">Est. slippage</span><span className="text-gold">${slippage.toFixed(2)}</span></div>
          <div className="flex justify-between"><span className="text-muted">Max loss (0.5%)</span><span className="text-red">${maxLoss.toFixed(2)}</span></div>
        </div>
      )}

      {/* Submit */}
      <button onClick={submit}
        className={clsx('w-full py-2 rounded-lg font-bold text-sm border transition-all',
          submitted
            ? 'border-green/60 bg-green/10 text-green'
            : side === 'BUY'
              ? 'border-green/60 bg-green/20 text-green hover:bg-green/30'
              : 'border-red/60 bg-red/20 text-red hover:bg-red/30')}>
        {submitted ? '✓ Submitted' : `${side} ${symbol}`}
      </button>

      {/* Current price */}
      <p className="text-center text-xs text-muted font-mono">
        {symbol}: <span className="text-text">${currentPrice.toLocaleString('en-US', { maximumFractionDigits: 2 })}</span>
      </p>
    </div>
  )
}
