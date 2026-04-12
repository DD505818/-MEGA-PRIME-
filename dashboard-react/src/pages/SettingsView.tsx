import { useState } from 'react'
import { useAppStore } from '../store/useAppStore'

export function SettingsView() {
  const { mode, setMode } = useAppStore()
  const [tab, setTab] = useState('general')
  const tabs = ['general', 'risk', 'api-keys', 'wallets', 'layout', 'compliance']

  return (
    <div className="p-3 flex flex-col gap-3">
      <div className="flex gap-1 border-b border-border pb-2">
        {tabs.map((t) => (
          <button key={t} onClick={() => setTab(t)}
            className={`tab ${tab === t ? 'tab-active' : ''} capitalize`}>
            {t.replace('-', ' ')}
          </button>
        ))}
      </div>
      <div className="card max-w-lg">
        {tab === 'general' && (
          <div className="space-y-4">
            <div>
              <label className="kicker">Trading Mode</label>
              <div className="flex gap-2 mt-1">
                {(['paper', 'live'] as const).map((m) => (
                  <button key={m} onClick={() => setMode(m)}
                    className={`px-4 py-2 rounded-lg border text-sm font-bold capitalize transition-all ${mode === m
                      ? m === 'live' ? 'border-red/60 bg-red/10 text-red' : 'border-green/60 bg-green/10 text-green'
                      : 'border-border text-muted hover:border-border/80'}`}>
                    {m}
                  </button>
                ))}
              </div>
            </div>
            <div>
              <label className="kicker">Theme</label>
              <select className="bg-panel2 border border-border rounded-lg px-3 py-2 text-sm text-text outline-none mt-1 block">
                <option>Dark (default)</option>
                <option>High Contrast</option>
                <option>OLED (true black)</option>
              </select>
            </div>
          </div>
        )}
        {tab === 'risk' && (
          <div className="space-y-3">
            {[
              { label: 'Risk per trade', value: '0.5%', readOnly: true },
              { label: 'Daily loss cap', value: '2.0%', readOnly: true },
              { label: 'Max drawdown', value: '10.0%', readOnly: true },
              { label: 'Max leverage', value: '10x', readOnly: false },
            ].map((r) => (
              <div key={r.label} className="flex items-center justify-between">
                <label className="text-sm text-text">{r.label}</label>
                <input defaultValue={r.value} readOnly={r.readOnly}
                  className="w-24 text-right bg-panel2 border border-border rounded px-2 py-1 text-sm font-mono text-text outline-none focus:border-blue/50" />
              </div>
            ))}
            <p className="text-xs text-muted">Core risk limits are hard-coded and cannot be changed without admin override.</p>
          </div>
        )}
        {tab === 'api-keys' && (
          <div className="space-y-3">
            {['Kraken', 'Binance', 'Coinbase'].map((exchange) => (
              <div key={exchange}>
                <label className="kicker">{exchange} API Key</label>
                <div className="flex gap-2 mt-1">
                  <input type="password" placeholder={`${exchange} API key...`}
                    className="flex-1 bg-panel2 border border-border rounded-lg px-3 py-1.5 text-sm text-text outline-none focus:border-blue/50" />
                  <button className="btn-primary text-xs">Validate</button>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
