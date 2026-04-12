import { useAppStore } from '../../store/useAppStore'
import clsx from 'clsx'

const AGENT_LABELS: Record<string, string> = {
  ceo: 'CEO', cfo: 'CFO', mamba: 'MAMBA', surge: 'SURGE', arb: 'ARB',
  opt: 'OPT', gold: 'GOLD', guard: 'GUARD', maker: 'MAKER', twin: 'TWIN',
  nexus: 'NEXUS', senti: 'SENTI', orbit: 'ORBIT', oracle: 'ORACLE',
  harvest: 'HARVEST', stake: 'STAKE', 'mev-hunter': 'MEV', midas: 'MIDAS', 'hi-darts': 'HI-DARTS',
}

export function AgentArmy() {
  const { agents, toggleAgent } = useAppStore()

  return (
    <div className="card h-full flex flex-col gap-2">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-semibold text-text">Agent Army</h3>
        <div className="flex gap-1">
          <button className="btn-primary text-xs py-0.5 px-2">Enable All</button>
          <button className="btn-danger text-xs py-0.5 px-2">Halt All</button>
        </div>
      </div>

      <div className="grid grid-cols-1 gap-1 overflow-y-auto flex-1">
        {agents.map((agent) => (
          <div key={agent.name}
            className="flex items-center justify-between p-2 rounded-lg border border-border/50 bg-panel2/60 hover:border-border transition-colors">
            <div className="flex items-center gap-2">
              {/* Toggle */}
              <button
                onClick={() => toggleAgent(agent.name)}
                className={clsx(
                  'w-8 h-4 rounded-full transition-all relative',
                  agent.active ? 'bg-green/30 border border-green/50' : 'bg-border/40 border border-border'
                )}>
                <span className={clsx(
                  'absolute top-0.5 w-3 h-3 rounded-full transition-all',
                  agent.active ? 'left-4 bg-green' : 'left-0.5 bg-muted'
                )} />
              </button>
              <span className={clsx(
                'text-xs font-bold font-mono',
                agent.name === 'ceo' ? 'text-gold' : agent.name === 'cfo' ? 'text-purple' : 'text-text'
              )}>
                {AGENT_LABELS[agent.name] ?? agent.name.toUpperCase()}
              </span>
              {agent.name === 'ceo' && <span className="text-xs">👑</span>}
            </div>
            <div className="flex items-center gap-3 text-xs font-mono text-muted">
              <span title="Win Rate">W:{agent.winRate.toFixed(0)}%</span>
              <span title="Sharpe">S:{agent.sharpe.toFixed(1)}</span>
              <span className={clsx(agent.pnl >= 0 ? 'text-green' : 'text-red')} title="P&L">
                {agent.pnl >= 0 ? '+' : ''}{agent.pnl.toFixed(0)}
              </span>
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
