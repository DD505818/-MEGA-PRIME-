import { useAppStore } from '../../store/useAppStore'
import { api } from '../../services/api'
import clsx from 'clsx'

const TICKER_SYMBOLS = ['BTC/USD', 'ETH/USD', 'SOL/USD', 'AVAX/USD', 'BNB/USD', 'XAU/USD']

export function TopBar() {
  const { mode, ticks, killSwitchActive, setKillSwitch } = useAppStore()

  const handleKill = async () => {
    await api.killSwitch()
    setKillSwitch(true)
  }

  return (
    <header className="flex items-center justify-between px-4 py-2 border-b border-border bg-panel/80 backdrop-blur sticky top-0 z-50">
      {/* Brand */}
      <div className="flex items-center gap-3">
        <span className="text-blue font-bold text-xl tracking-widest">Ω</span>
        <span className="font-bold tracking-wide">OMEGA PRIME</span>
        <span className={clsx('text-xs font-bold px-2 py-0.5 rounded-full border',
          mode === 'live'
            ? 'text-green border-green/40 bg-green/10'
            : 'text-gold border-gold/40 bg-gold/10')}>
          {mode === 'live' ? 'LIVE' : 'PAPER'}
        </span>
      </div>

      {/* Live ticker */}
      <div className="hidden lg:flex items-center gap-4 text-sm font-mono">
        {TICKER_SYMBOLS.map((sym) => {
          const t = ticks[sym]
          if (!t) return <span key={sym} className="text-muted">{sym}</span>
          const pos = t.change_24h >= 0
          return (
            <div key={sym} className="flex flex-col items-end">
              <span className="text-muted text-xs">{sym.split('/')[0]}</span>
              <span className={pos ? 'text-green' : 'text-red'}>
                ${t.price.toLocaleString('en-US', { maximumFractionDigits: 2 })}
              </span>
            </div>
          )
        })}
      </div>

      {/* Controls */}
      <div className="flex items-center gap-2">
        <button className="btn-primary text-xs">Start All</button>
        <button className="btn-danger text-xs">Stop All</button>
        <button
          onClick={handleKill}
          disabled={killSwitchActive}
          className={clsx(
            'px-3 py-1.5 rounded-full text-xs font-bold border transition-all',
            killSwitchActive
              ? 'border-green/60 text-green bg-green/10 cursor-not-allowed'
              : 'border-red animate-flash text-red hover:bg-red hover:text-white'
          )}>
          {killSwitchActive ? 'SAFE' : 'KILL SWITCH'}
        </button>
        <div className="w-7 h-7 rounded-full bg-blue/20 border border-blue/40 flex items-center justify-center text-xs font-bold text-blue">
          U
        </div>
      </div>
    </header>
  )
}
