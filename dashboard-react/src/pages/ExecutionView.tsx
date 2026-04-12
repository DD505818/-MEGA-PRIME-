import { OrderEntry } from '../components/widgets/OrderEntry'

const MOCK_ORDERS = [
  { id: '#560021', symbol: 'BTC/USD', type: 'LIMIT',  side: 'BUY',  qty: 0.5,  price: 67284, status: 'OPEN' },
  { id: '#560228', symbol: 'ETH/USD', type: 'MARKET', side: 'BUY',  qty: 2.0,  price: 55054, status: 'FILLED' },
  { id: '#560198', symbol: 'SOL/USD', type: 'TWAP',   side: 'SELL', qty: 10.0, price: 925,   status: 'PARTIAL' },
]

const statusColor = { OPEN: 'text-blue', FILLED: 'text-green', PARTIAL: 'text-gold', CANCELLED: 'text-red' }

export function ExecutionView() {
  return (
    <div className="p-3 grid grid-cols-3 gap-3">
      <OrderEntry />
      <div className="col-span-2 card">
        <p className="kicker mb-3">Order Management</p>
        <table className="w-full text-xs font-mono">
          <thead>
            <tr className="text-muted border-b border-border">
              <th className="text-left py-1">ID</th>
              <th className="text-left">Symbol</th>
              <th className="text-left">Type</th>
              <th className="text-left">Side</th>
              <th className="text-right">Qty</th>
              <th className="text-right">Price</th>
              <th className="text-right">Status</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {MOCK_ORDERS.map((o) => (
              <tr key={o.id} className="border-b border-border/30 hover:bg-panel2/50">
                <td className="py-2 text-muted">{o.id}</td>
                <td className="text-text font-bold">{o.symbol}</td>
                <td className="text-muted">{o.type}</td>
                <td className={o.side === 'BUY' ? 'text-green' : 'text-red'}>{o.side}</td>
                <td className="text-right">{o.qty}</td>
                <td className="text-right">${o.price.toLocaleString()}</td>
                <td className={`text-right ${statusColor[o.status as keyof typeof statusColor]}`}>{o.status}</td>
                <td className="text-right">
                  {o.status === 'OPEN' && (
                    <button className="text-xs text-red border border-red/30 px-1.5 py-0.5 rounded hover:bg-red/10">Cancel</button>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}
