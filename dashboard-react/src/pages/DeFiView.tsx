const WALLETS = [
  { name: 'MetaMask',     balance: '3.2 ETH',   usd: '$10,054',  type: 'EVM' },
  { name: 'WalletConnect', balance: '12 SOL',    usd: '$2,069',   type: 'SOL' },
  { name: 'Ledger',       balance: '0.5 BTC',    usd: '$32,115',  type: 'EVM' },
  { name: 'Trezor',       balance: '45 AVAX',    usd: '$1,755',   type: 'EVM' },
  { name: 'Phantom',      balance: '150 DOT',    usd: '$1,095',   type: 'SOL' },
]
const YIELDS = [
  { asset: 'ETH',  apy: '4.2%',  lock: 'None',  platform: 'Lido' },
  { asset: 'SOL',  apy: '6.8%',  lock: 'None',  platform: 'Marinade' },
  { asset: 'DOT',  apy: '14.5%', lock: '28d',   platform: 'Native' },
  { asset: 'AVAX', apy: '8.9%',  lock: 'None',  platform: 'Avalanche' },
]

export function DeFiView() {
  return (
    <div className="p-3 grid grid-cols-2 gap-3">
      <div className="card">
        <p className="kicker mb-3">Connected Wallets</p>
        <div className="space-y-2">
          {WALLETS.map((w) => (
            <div key={w.name} className="flex items-center justify-between border border-border/50 rounded-lg p-2 text-sm">
              <div>
                <p className="font-bold text-text">{w.name}</p>
                <p className="text-xs text-muted">{w.type}</p>
              </div>
              <div className="text-right">
                <p className="font-mono text-text">{w.balance}</p>
                <p className="text-xs text-muted">{w.usd}</p>
              </div>
            </div>
          ))}
          <button className="w-full py-2 text-sm border border-blue/40 text-blue rounded-lg hover:bg-blue/10 transition-all">
            + Connect Wallet
          </button>
        </div>
      </div>
      <div className="card">
        <p className="kicker mb-3">Staking & Yield</p>
        <div className="space-y-2">
          {YIELDS.map((y) => (
            <div key={y.asset} className="flex items-center justify-between border border-border/50 rounded-lg p-2 text-sm">
              <div>
                <p className="font-bold text-text">{y.asset}</p>
                <p className="text-xs text-muted">{y.platform} · Lock: {y.lock}</p>
              </div>
              <div className="flex items-center gap-3">
                <span className="text-green font-bold font-mono">{y.apy}</span>
                <button className="text-xs border border-green/40 text-green px-2 py-0.5 rounded hover:bg-green/10">Stake</button>
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
