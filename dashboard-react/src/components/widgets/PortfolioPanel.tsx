import { useAppStore } from '../../store/useAppStore'
import { LineChart, Line, XAxis, YAxis, Tooltip, ResponsiveContainer } from 'recharts'
import { format } from 'date-fns'
import clsx from 'clsx'

export function PortfolioPanel() {
  const { equity, equityCurve, positions, dailyPnl } = useAppStore()

  const chartData = equityCurve.map((p) => ({
    time: format(p.ts, 'HH:mm'),
    value: Math.round(p.value),
  }))

  return (
    <div className="card flex flex-col gap-3">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-semibold text-text">Portfolio</h3>
        <span className={clsx('text-sm font-bold font-mono', dailyPnl >= 0 ? 'text-green' : 'text-red')}>
          {dailyPnl >= 0 ? '+' : ''}${dailyPnl.toFixed(2)} today
        </span>
      </div>

      {/* Equity */}
      <div>
        <p className="kicker">Total Equity</p>
        <p className="text-2xl font-bold font-mono text-text">${equity.toLocaleString('en-US', { maximumFractionDigits: 0 })}</p>
      </div>

      {/* Equity curve */}
      <ResponsiveContainer width="100%" height={80}>
        <LineChart data={chartData}>
          <XAxis dataKey="time" hide />
          <YAxis hide domain={['auto', 'auto']} />
          <Tooltip
            contentStyle={{ background: '#0f1828', border: '1px solid #263753', borderRadius: 8, fontSize: 11 }}
            formatter={(v: number) => [`$${v.toLocaleString()}`, 'Equity']} />
          <Line type="monotone" dataKey="value" stroke="#4df2b2" strokeWidth={1.5} dot={false} />
        </LineChart>
      </ResponsiveContainer>

      {/* Positions */}
      <div>
        <p className="kicker mb-1">Open Positions</p>
        <div className="space-y-1">
          {positions.map((p) => (
            <div key={p.symbol}
              className="flex items-center justify-between text-xs font-mono p-1.5 rounded border border-border/40 bg-panel2/50">
              <div className="flex items-center gap-2">
                <span className={clsx('px-1 py-0.5 rounded text-xs font-bold',
                  p.side === 'LONG' ? 'bg-green/10 text-green border border-green/30' : 'bg-red/10 text-red border border-red/30')}>
                  {p.side}
                </span>
                <span className="text-text">{p.symbol}</span>
                <span className="text-muted">{p.qty}</span>
              </div>
              <div className="text-right">
                <span className={p.pnl >= 0 ? 'text-green' : 'text-red'}>
                  {p.pnl >= 0 ? '+' : ''}${p.pnl.toFixed(2)}
                </span>
                <span className="text-muted ml-1">({p.pnlPct >= 0 ? '+' : ''}{p.pnlPct.toFixed(2)}%)</span>
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
