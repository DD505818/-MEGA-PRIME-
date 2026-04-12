import { useEffect, useRef, useState } from 'react'
import { createChart, ColorType, CrosshairMode } from 'lightweight-charts'
import { useAppStore } from '../../store/useAppStore'

const TIMEFRAMES = ['1m', '5m', '15m', '1h', '4h', '1D']
const SYMBOLS = ['BTC/USD', 'ETH/USD', 'SOL/USD', 'AVAX/USD']

function generateCandles(symbol: string, count = 100) {
  const base: Record<string, number> = { 'BTC/USD': 64231, 'ETH/USD': 3142, 'SOL/USD': 172, 'AVAX/USD': 38 }
  let price = base[symbol] ?? 100
  const now = Math.floor(Date.now() / 1000)
  return Array.from({ length: count }, (_, i) => {
    const open  = price
    const close = price * (1 + (Math.random() - 0.48) * 0.012)
    const high  = Math.max(open, close) * (1 + Math.random() * 0.005)
    const low   = Math.min(open, close) * (1 - Math.random() * 0.005)
    price = close
    return { time: now - (count - i) * 60, open, high, low, close, value: Math.random() * 10 + 1 }
  })
}

export function InteractiveChart() {
  const chartRef  = useRef<HTMLDivElement>(null)
  const chartObj  = useRef<ReturnType<typeof createChart> | null>(null)
  const candleSeries = useRef<any>(null)
  const [symbol, setSymbol] = useState('BTC/USD')
  const [tf, setTf] = useState('1m')
  const ticks = useAppStore((s) => s.ticks)

  useEffect(() => {
    if (!chartRef.current) return
    const chart = createChart(chartRef.current, {
      layout: { background: { type: ColorType.Solid, color: '#0f1828' }, textColor: '#9fb0cc' },
      grid: { vertLines: { color: '#263753' }, horzLines: { color: '#263753' } },
      crosshair: { mode: CrosshairMode.Normal },
      timeScale: { borderColor: '#263753', timeVisible: true, secondsVisible: false },
      rightPriceScale: { borderColor: '#263753' },
      width: chartRef.current.offsetWidth,
      height: 300,
    })
    chartObj.current = chart

    const candles = chart.addCandlestickSeries({
      upColor: '#4df2b2', downColor: '#ff617d',
      borderUpColor: '#4df2b2', borderDownColor: '#ff617d',
      wickUpColor: '#4df2b2', wickDownColor: '#ff617d',
    })
    candleSeries.current = candles
    candles.setData(generateCandles(symbol))

    // Box Theory levels (PDH / PDL / midpoint)
    const prices = generateCandles(symbol, 20).map((c) => c.close)
    const pdh = Math.max(...prices)
    const pdl = Math.min(...prices)
    const mid = (pdh + pdl) / 2

    ;[
      { price: pdh, color: '#6dbbff', title: 'PDH', dashed: false },
      { price: mid, color: '#9fb0cc', title: 'MID', dashed: true },
      { price: pdl, color: '#ffc27a', title: 'PDL', dashed: false },
    ].forEach(({ price, color, title }) => {
      candles.createPriceLine({ price, color, lineWidth: 1, lineStyle: 2, axisLabelVisible: true, title })
    })

    const ro = new ResizeObserver(() => {
      chart.applyOptions({ width: chartRef.current!.offsetWidth })
    })
    ro.observe(chartRef.current)
    return () => { ro.disconnect(); chart.remove() }
  }, [symbol])

  // Live tick updates
  useEffect(() => {
    const tick = ticks[symbol]
    if (!tick || !candleSeries.current) return
    const t = Math.floor(tick.ts)
    candleSeries.current.update({ time: t, open: tick.bid, high: tick.ask, low: tick.bid * 0.9999, close: tick.price })
  }, [ticks, symbol])

  return (
    <div className="card flex flex-col gap-2">
      <div className="flex items-center justify-between">
        <div className="flex gap-1">
          {SYMBOLS.map((s) => (
            <button key={s} onClick={() => setSymbol(s)}
              className={`px-2 py-0.5 rounded text-xs font-mono transition-colors ${s === symbol ? 'bg-blue/20 text-blue border border-blue/40' : 'text-muted hover:text-text'}`}>
              {s.split('/')[0]}
            </button>
          ))}
        </div>
        <div className="flex gap-1">
          {TIMEFRAMES.map((t) => (
            <button key={t} onClick={() => setTf(t)}
              className={`px-2 py-0.5 rounded text-xs transition-colors ${t === tf ? 'bg-panel2 text-text border border-border' : 'text-muted hover:text-text'}`}>
              {t}
            </button>
          ))}
        </div>
      </div>
      <div ref={chartRef} className="w-full" />
    </div>
  )
}
