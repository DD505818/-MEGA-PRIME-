import { Area, AreaChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts';
import { orderBookAsks, orderBookBids } from '../data/mockData';
import { Panel } from './Panel';

const depth = [...orderBookBids.map((d) => ({ ...d, side: 'bid' })), ...orderBookAsks.map((d) => ({ ...d, side: 'ask' }))];

export function OrderBook() {
  return (
    <Panel title="Order Book" subtitle="L2 depth + top of book">
      <div className="orderbook-grid">
        <div>
          <h3>Bids</h3>
          {orderBookBids.map((row) => (
            <div key={row.price} className="order-row bid">
              <span>{row.price.toLocaleString()}</span>
              <span>{row.size.toFixed(2)}</span>
            </div>
          ))}
        </div>
        <div>
          <h3>Asks</h3>
          {orderBookAsks.map((row) => (
            <div key={row.price} className="order-row ask">
              <span>{row.price.toLocaleString()}</span>
              <span>{row.size.toFixed(2)}</span>
            </div>
          ))}
        </div>
      </div>
      <div style={{ height: 180 }}>
        <ResponsiveContainer width="100%" height="100%">
          <AreaChart data={depth}>
            <XAxis dataKey="price" hide />
            <YAxis hide />
            <Tooltip />
            <Area type="stepAfter" dataKey="size" stroke="#a67dff" fill="#a67dff3b" />
          </AreaChart>
        </ResponsiveContainer>
      </div>
    </Panel>
  );
}
