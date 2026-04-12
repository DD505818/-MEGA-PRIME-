import { useRef, useState } from 'react'
import { useAppStore } from '../../store/useAppStore'
import { api } from '../../services/api'
import clsx from 'clsx'

function Gauge({ label, value, max, warn = 0.7, danger = 0.9 }: {
  label: string; value: number; max: number; warn?: number; danger?: number
}) {
  const pct = Math.min(value / max, 1)
  const color = pct >= danger ? 'stroke-red' : pct >= warn ? 'stroke-gold' : 'stroke-green'
  const r = 36, circ = 2 * Math.PI * r, arc = circ * 0.75
  const offset = arc - arc * pct
  return (
    <div className="flex flex-col items-center gap-1">
      <svg width="90" height="90" viewBox="0 0 90 90">
        <circle cx="45" cy="45" r={r} fill="none" stroke="#263753" strokeWidth="8"
          strokeDasharray={`${arc} ${circ - arc}`} strokeDashoffset={-circ * 0.125}
          transform="rotate(-225 45 45)" />
        <circle cx="45" cy="45" r={r} fill="none" strokeWidth="8"
          className={color}
          strokeDasharray={`${arc} ${circ - arc}`}
          strokeDashoffset={offset + circ * 0.125}
          strokeLinecap="round"
          transform="rotate(-225 45 45)"
          style={{ transition: 'stroke-dashoffset 0.5s ease' }} />
        <text x="45" y="48" textAnchor="middle" className="fill-text text-xs font-mono font-bold" fontSize="12">
          {(value * 100).toFixed(1)}%
        </text>
      </svg>
      <span className="text-xs text-muted">{label}</span>
    </div>
  )
}

export function RiskPanel() {
  const { dailyLossPct, drawdownPct, varValue, killSwitchActive, setKillSwitch } = useAppStore()
  const [holdProgress, setHoldProgress] = useState(0)
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null)

  const startHold = () => {
    if (killSwitchActive) return
    let p = 0
    timerRef.current = setInterval(() => {
      p += 5
      setHoldProgress(p)
      if (p >= 100) {
        clearInterval(timerRef.current!)
        api.killSwitch()
        setKillSwitch(true)
        setHoldProgress(0)
      }
    }, 100)
  }

  const endHold = () => {
    if (timerRef.current) clearInterval(timerRef.current)
    setHoldProgress(0)
  }

  return (
    <div className="card flex flex-col gap-3">
      <h3 className="text-sm font-semibold text-text">Risk Panel</h3>

      {/* Kill Switch */}
      <div className="flex justify-center">
        <div className="relative">
          <button
            onMouseDown={startHold} onMouseUp={endHold} onMouseLeave={endHold}
            onTouchStart={startHold} onTouchEnd={endHold}
            disabled={killSwitchActive}
            className={clsx(
              'w-24 h-24 rounded-full font-bold text-sm border-2 transition-all select-none',
              killSwitchActive
                ? 'border-green text-green bg-green/10 cursor-not-allowed'
                : 'border-red text-red bg-red/10 hover:bg-red/20 cursor-pointer animate-flash'
            )}>
            {killSwitchActive ? 'SAFE' : 'KILL\nSWITCH'}
          </button>
          {holdProgress > 0 && (
            <svg className="absolute inset-0 -rotate-90 pointer-events-none" width="96" height="96">
              <circle cx="48" cy="48" r="46" fill="none" stroke="#ff617d" strokeWidth="4"
                strokeDasharray={`${holdProgress * 2.89} 289`} />
            </svg>
          )}
        </div>
      </div>
      {!killSwitchActive && <p className="text-center text-xs text-muted">Hold 2s to activate</p>}

      {/* Gauges */}
      <div className="flex justify-around">
        <Gauge label="Daily Loss" value={dailyLossPct / 100} max={0.02} warn={0.7} danger={0.9} />
        <Gauge label="Drawdown" value={drawdownPct / 100} max={0.10} warn={0.6} danger={0.85} />
      </div>

      {/* VaR */}
      <div className="border border-border/50 rounded-lg p-2 text-center">
        <p className="kicker">VaR (95%)</p>
        <p className="font-mono font-bold text-gold text-lg">${varValue.toFixed(2)}</p>
      </div>
    </div>
  )
}
