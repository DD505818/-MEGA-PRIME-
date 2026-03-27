import { Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts';
import { pnlSeries } from '../data/mockData';
import { Panel } from './Panel';

export function PortfolioPanel() {
  const latest = pnlSeries.at(-1)?.pnl ?? 0;

  return (
    <Panel title="Portfolio" subtitle="PnL and exposure">
      <div className="kpi-row">
        <div><span>Net P&L</span><strong>${latest.toLocaleString()}</strong></div>
        <div><span>Exposure</span><strong>$1.42M</strong></div>
        <div><span>Sharpe (30d)</span><strong>1.94</strong></div>
      </div>
      <div style={{ height: 160 }}>
        <ResponsiveContainer width="100%" height="100%">
          <LineChart data={pnlSeries}>
            <XAxis dataKey="time" stroke="#bddfff" />
            <YAxis stroke="#bddfff" />
            <Tooltip />
            <Line type="monotone" dataKey="pnl" stroke="#34f6ff" strokeWidth={2} dot={false} />
          </LineChart>
        </ResponsiveContainer>
      </div>
    </Panel>
  );
}
