import { tickerData } from '../data/mockData';

export function LiveMarketTicker() {
  return (
    <div className="ticker-wrap">
      <div className="ticker-track">
        {[...tickerData, ...tickerData].map((item, idx) => (
          <div key={`${item.symbol}-${idx}`} className="ticker-item">
            <span>{item.symbol}</span>
            <strong>${item.price.toLocaleString()}</strong>
            <em className={item.change >= 0 ? 'up' : 'down'}>{item.change >= 0 ? '+' : ''}{item.change}%</em>
          </div>
        ))}
      </div>
    </div>
  );
}
