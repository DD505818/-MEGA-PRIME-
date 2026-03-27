import { useState } from 'react';
import { Panel } from './Panel';

export function OrderEntryPanel() {
  const [side, setSide] = useState<'buy' | 'sell'>('buy');

  return (
    <Panel title="Order Entry" subtitle="Execution router bridge">
      <form className="order-form" onSubmit={(e) => e.preventDefault()}>
        <div className="segmented">
          <button type="button" className={side === 'buy' ? 'active buy' : ''} onClick={() => setSide('buy')}>Buy</button>
          <button type="button" className={side === 'sell' ? 'active sell' : ''} onClick={() => setSide('sell')}>Sell</button>
        </div>
        <label>
          Symbol
          <input defaultValue="BTC/USD" />
        </label>
        <label>
          Quantity
          <input type="number" defaultValue="0.25" step="0.01" />
        </label>
        <label>
          Limit Price
          <input type="number" defaultValue="68420" step="1" />
        </label>
        <button className="primary-btn" type="submit">Submit {side.toUpperCase()} Order</button>
      </form>
    </Panel>
  );
}
