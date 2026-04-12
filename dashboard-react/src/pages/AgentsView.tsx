import { AgentArmy }     from '../components/widgets/AgentArmy'
import { AnomalyMonitor } from '../components/widgets/AnomalyMonitor'
import { LLMChat }         from '../components/widgets/LLMChat'
import { useAppStore }     from '../store/useAppStore'
import clsx from 'clsx'

export function AgentsView() {
  const { agents } = useAppStore()
  const active = agents.filter((a) => a.active).length
  const totalPnl = agents.reduce((s, a) => s + a.pnl, 0)

  return (
    <div className="p-3 flex flex-col gap-3">
      {/* Summary row */}
      <div className="grid grid-cols-4 gap-3">
        {[
          { label: 'Active Agents', value: `${active}/19`, color: 'text-green' },
          { label: 'Total PnL', value: `${totalPnl >= 0 ? '+' : ''}$${totalPnl.toFixed(0)}`, color: totalPnl >= 0 ? 'text-green' : 'text-red' },
          { label: 'Avg Sharpe', value: (agents.reduce((s, a) => s + a.sharpe, 0) / agents.length).toFixed(2), color: 'text-blue' },
          { label: 'Avg Win Rate', value: `${(agents.reduce((s, a) => s + a.winRate, 0) / agents.length).toFixed(1)}%`, color: 'text-gold' },
        ].map((m) => (
          <div key={m.label} className="card text-center">
            <p className="kicker">{m.label}</p>
            <p className={clsx('metric-value', m.color)}>{m.value}</p>
          </div>
        ))}
      </div>

      <div className="grid grid-cols-3 gap-3">
        <div className="col-span-2"><AgentArmy /></div>
        <div className="flex flex-col gap-3">
          <AnomalyMonitor />
          <LLMChat />
        </div>
      </div>
    </div>
  )
}
