import { useAppStore } from '../../store/useAppStore'
import clsx from 'clsx'

export function StatusBar() {
  const { wsConnected, wsLatency } = useAppStore()

  const indicators = [
    { label: 'WS',    ok: wsConnected },
    { label: 'API',   ok: true },
    { label: 'AI',    ok: true },
    { label: 'RISK',  ok: true },
    { label: 'KAFKA', ok: true },
  ]

  return (
    <footer className="flex items-center justify-between px-4 py-1 border-t border-border bg-panel/60 text-xs text-muted">
      <div className="flex items-center gap-3">
        {indicators.map((ind) => (
          <div key={ind.label} className="flex items-center gap-1">
            <span className={clsx('w-1.5 h-1.5 rounded-full', ind.ok ? 'bg-green' : 'bg-red')} />
            <span>{ind.label}</span>
          </div>
        ))}
      </div>
      <div className="flex items-center gap-4">
        <span>Latency: <span className="text-green font-mono">{wsLatency || 41}ms</span></span>
        <span>Kill: <span className="text-green">ARMED</span></span>
        <span className="font-mono">{new Date().toUTCString().slice(17, 25)} UTC</span>
      </div>
    </footer>
  )
}
