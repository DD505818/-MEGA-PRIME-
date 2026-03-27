import { defiTokens } from '../data/mockData';
import { Panel } from './Panel';

export function DeFiDashboard() {
  return (
    <Panel title="DeFi Dashboard" subtitle="On-chain balances and yields">
      <div className="token-table">
        <div className="token-head"><span>Token</span><span>Balance</span><span>APY</span></div>
        {defiTokens.map((token) => (
          <div key={token.token} className="token-row">
            <span>{token.token}</span>
            <span>{token.balance.toLocaleString()}</span>
            <span>{token.apy}%</span>
          </div>
        ))}
      </div>
    </Panel>
  );
}
