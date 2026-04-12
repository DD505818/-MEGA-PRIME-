import { useAppStore } from '../../store/useAppStore'
import { format } from 'date-fns'
import clsx from 'clsx'

export function AnomalyMonitor() {
  const { anomalies, acknowledgeAnomaly } = useAppStore()

  const severityColor = { low: 'text-blue border-blue/30', medium: 'text-gold border-gold/30', high: 'text-red border-red/30' }

  return (
    <div className="card flex flex-col gap-2">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-semibold text-text">Anomaly Monitor</h3>
        <span className="text-xs font-mono text-muted">{anomalies.filter((a) => !a.acknowledged).length} active</span>
      </div>
      <div className="space-y-1 overflow-y-auto max-h-48">
        {anomalies.length === 0 && <p className="text-xs text-muted text-center py-4">No anomalies detected</p>}
        {anomalies.map((a) => (
          <div key={a.id}
            className={clsx('flex items-start justify-between p-2 rounded-lg border text-xs',
              a.acknowledged ? 'opacity-40' : '', severityColor[a.severity])}>
            <div className="flex-1">
              <div className="flex items-center gap-2 mb-0.5">
                <span className="font-bold font-mono">{a.type}</span>
                <span className={clsx('px-1.5 py-0.5 rounded-full text-xs border uppercase',
                  { low: 'text-blue border-blue/30 bg-blue/10', medium: 'text-gold border-gold/30 bg-gold/10', high: 'text-red border-red/30 bg-red/10' }[a.severity])}>
                  {a.severity}
                </span>
              </div>
              <p className="text-muted">{a.message}</p>
              <p className="text-muted/60 mt-0.5">{format(a.ts, 'HH:mm:ss')}</p>
            </div>
            {!a.acknowledged && (
              <button onClick={() => acknowledgeAnomaly(a.id)}
                className="ml-2 text-xs text-muted hover:text-text border border-border/50 px-2 py-0.5 rounded">
                ACK
              </button>
            )}
          </div>
        ))}
      </div>
    </div>
  )
}
