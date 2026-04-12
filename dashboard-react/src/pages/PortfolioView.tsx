import { PortfolioPanel } from '../components/widgets/PortfolioPanel'
import { useAppStore }    from '../store/useAppStore'
import clsx from 'clsx'

export function PortfolioView() {
  const { positions, dailyPnl, equity } = useAppStore()
  return (
    <div className="p-3 flex flex-col gap-3">
      <div className="grid grid-cols-3 gap-3">
        {[
          { label: 'Total Equity',   value: `$${equity.toLocaleString()}`,                color: 'text-text' },
          { label: 'Daily PnL',      value: `${dailyPnl >= 0 ? '+' : ''}$${dailyPnl.toFixed(2)}`, color: dailyPnl >= 0 ? 'text-green' : 'text-red' },
          { label: 'Open Positions', value: String(positions.length),                     color: 'text-blue' },
        ].map((m) => (
          <div key={m.label} className="card text-center">
            <p className="kicker">{m.label}</p>
            <p className={clsx('metric-value', m.color)}>{m.value}</p>
          </div>
        ))}
      </div>
      <PortfolioPanel />
    </div>
  )
}
