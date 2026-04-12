import { useAppStore } from '../../store/useAppStore'
import clsx from 'clsx'

const WATCHLIST = ['BTC/USD', 'ETH/USD', 'SOL/USD', 'AVAX/USD', 'BNB/USD', 'XAU/USD', 'AAPL', 'SPX']

export function Watchlist() {
  const ticks = useAppStore((s) => s.ticks)

  return (
    <div className="card flex flex-col gap-1 h-full overflow-y-auto">
      <h3 className="text-sm font-semibold text-text mb-1">Watchlist</h3>
      {WATCHLIST.map((sym) => {
        const t = ticks[sym]
        const pos = t ? t.change_24h >= 0 : true
        return (
          <div key={sym}
            className="flex items-center justify-between p-2 rounded-lg border border-border/30 hover:border-border/70 hover:bg-panel2/50 cursor-pointer transition-all">
            <div>
              <p className="text-xs font-bold font-mono text-text">{sym}</p>
              <p className="text-xs text-muted">Vol: {t ? t.volume.toFixed(2) : '—'}</p>
            </div>
            <div className="text-right">
              <p className="text-xs font-mono font-bold text-text">
                {t ? `$${t.price.toLocaleString('en-US', { maximumFractionDigits: 2 })}` : '—'}
              </p>
              <p className={clsx('text-xs font-mono', pos ? 'text-green' : 'text-red')}>
                {t ? `${pos ? '+' : ''}${(t.change_24h * 100).toFixed(2)}%` : '—'}
              </p>
            </div>
          </div>
        )
      })}
    </div>
  )
}
