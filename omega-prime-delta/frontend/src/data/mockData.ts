export const tickerData = [
  { symbol: 'BTC/USD', price: 68420, change: 1.82 },
  { symbol: 'ETH/USD', price: 3455, change: -0.63 },
  { symbol: 'SOL/USD', price: 184.42, change: 2.91 },
  { symbol: 'US30', price: 39420, change: 0.44 },
  { symbol: 'XAU/USD', price: 2364.15, change: -0.18 },
];

export const orderBookBids = [
  { price: 68410, size: 1.34 },
  { price: 68405, size: 2.56 },
  { price: 68400, size: 3.11 },
  { price: 68395, size: 4.82 },
  { price: 68390, size: 6.44 },
];

export const orderBookAsks = [
  { price: 68425, size: 1.02 },
  { price: 68430, size: 1.74 },
  { price: 68435, size: 2.31 },
  { price: 68440, size: 3.14 },
  { price: 68445, size: 4.22 },
];

export const pnlSeries = [
  { time: '09:30', pnl: 1200 },
  { time: '10:00', pnl: 1980 },
  { time: '10:30', pnl: 1640 },
  { time: '11:00', pnl: 2420 },
  { time: '11:30', pnl: 2850 },
  { time: '12:00', pnl: 3010 },
  { time: '12:30', pnl: 2780 },
  { time: '13:00', pnl: 3560 },
];

export const defiTokens = [
  { token: 'USDC', balance: 18240, apy: 5.1 },
  { token: 'wETH', balance: 12.4, apy: 3.8 },
  { token: 'stSOL', balance: 421, apy: 6.4 },
  { token: 'MATIC', balance: 10410, apy: 4.3 },
];

export const anomalies = [
  { id: 1, severity: 'high', label: 'Spread spike on BTC perpetuals', time: '2m ago' },
  { id: 2, severity: 'medium', label: 'Latency drift from NY4 gateway', time: '7m ago' },
  { id: 3, severity: 'low', label: 'DeFi wallet gas utilization > 80%', time: '13m ago' },
];
