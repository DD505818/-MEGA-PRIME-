import { RiskPanel }       from '../components/widgets/RiskPanel'
import { AnomalyMonitor }  from '../components/widgets/AnomalyMonitor'
import { useAppStore }     from '../store/useAppStore'

export function AnalyticsView() {
  const { agents, dailyLossPct, drawdownPct, varValue } = useAppStore()
  return (
    <div className="p-3 grid grid-cols-3 gap-3">
      <div className="col-span-1 flex flex-col gap-3">
        <RiskPanel />
        <AnomalyMonitor />
      </div>
      <div className="col-span-2 card">
        <p className="kicker mb-3">Agent Performance</p>
        <div className="overflow-x-auto">
          <table className="w-full text-xs font-mono">
            <thead>
              <tr className="text-muted border-b border-border">
                <th className="text-left py-1">Agent</th>
                <th className="text-right">Sharpe</th>
                <th className="text-right">Win Rate</th>
                <th className="text-right">Trades</th>
                <th className="text-right">PnL</th>
              </tr>
            </thead>
            <tbody>
              {agents.map((a) => (
                <tr key={a.name} className="border-b border-border/30 hover:bg-panel2/50">
                  <td className="py-1.5 text-text font-bold">{a.name.toUpperCase()}</td>
                  <td className="text-right text-blue">{a.sharpe.toFixed(2)}</td>
                  <td className="text-right text-text">{a.winRate.toFixed(1)}%</td>
                  <td className="text-right text-muted">{a.trades}</td>
                  <td className={`text-right ${a.pnl >= 0 ? 'text-green' : 'text-red'}`}>
                    {a.pnl >= 0 ? '+' : ''}${a.pnl.toFixed(0)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  )
}
