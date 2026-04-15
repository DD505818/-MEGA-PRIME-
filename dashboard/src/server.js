const express = require("express");

const app = express();
const port = 3000;

const terminalSnapshot = {
  status: "LIVE",
  ticker: "BTCUSDT",
  price: "$42,310",
  dayPnl: "+$820",
  latency: "41ms"
};

const charts = ["BTCUSDT", "ETHUSDT", "SOLUSDT", "NASDAQ"];

const topPanels = [
  { name: "Market Watch", value: "24 pairs", trend: "+2.4% breadth" },
  { name: "Orderbook", value: "$18.2M", trend: "Bid wall active" },
  { name: "Liquidity Heatmap", value: "Dense", trend: "3 hotspot clusters" },
  { name: "Portfolio & Risk", value: "2.1 Sharpe", trend: "VaR within limits" },
  { name: "Wallets & Fund Mgmt", value: "$2.4M AUM", trend: "5 connected wallets" },
  { name: "Execution Router", value: "12ms p95", trend: "Smart venue balancing" }
];

const walletBalances = [
  { wallet: "MetaMask", amount: "3.2 ETH", usd: "$8,420" },
  { wallet: "WalletConnect", amount: "12 SOL", usd: "$1,740" },
  { wallet: "Ledger", amount: "0.5 BTC", usd: "$32,115" },
  { wallet: "Trezor", amount: "45 AVAX", usd: "$1,553" },
  { wallet: "Phantom", amount: "150 DOT", usd: "$1,095" }
];

const allocations = [
  { bucket: "Spot", percent: 45 },
  { bucket: "Futures", percent: 30 },
  { bucket: "Staking", percent: 15 },
  { bucket: "Yield", percent: 10 }
];

const predictions = [
  { symbol: "BTC", signal: "BULLISH", confidence: 95, target: "$44,200" },
  { symbol: "ETH", signal: "BULLISH", confidence: 88, target: "$2,460" },
  { symbol: "SOL", signal: "BULLISH", confidence: 72, target: "$172" },
  { symbol: "AVAX", signal: "BEARISH", confidence: 65, target: "$31" }
];

const sentiment = { bear: 22, neutral: 31, bull: 47, score: 74 };

const onChain = [
  { metric: "Exchange inflow", value: "-12.4%", trend: "↓" },
  { metric: "Whale wallets", value: "+18", trend: "↑" },
  { metric: "Active addresses", value: "1.34M", trend: "↑" },
  { metric: "Network hashrate", value: "612 EH/s", trend: "↑" },
  { metric: "Funding rates", value: "0.011%", trend: "↓" }
];

app.get("/api/widgets", (_req, res) => {
  const data = predictions;

  // Before mapping, validate incoming array data.
  if (!Array.isArray(data) || data.length === 0) {
    console.error("Invalid or empty data array");
    res.status(400).json({ error: "Invalid data" });
    return;
  }

  // Use a safe for loop to avoid broken map chains.
  const processed = [];
  for (let i = 0; i < data.length; i += 1) {
    const item = data[i];
    if (!item || typeof item !== "object") {
      continue;
    }

    processed.push({
      id: item.id || `temp-${i}`,
      value: item.value || 0,
      timestamp: item.timestamp || Date.now()
    });
  }

  res.json({
    app: "dashboard",
    terminalSnapshot,
    charts,
    topPanels,
    walletBalances,
    allocations,
    predictions,
    sentiment,
    onChain,
    processed
  });
});

app.get("/", (_req, res) => {
  const chartTiles = charts.map((symbol) => `<article class="chart-card"><h3>${symbol}</h3><div class="live-line"></div></article>`).join("");
  const topPanelTiles = topPanels
    .map((panel) => `<article class="panel"><p class="kicker">${panel.name}</p><h4>${panel.value}</h4><p>${panel.trend}</p></article>`)
    .join("");

  const walletRows = walletBalances
    .map((item) => `<li><strong>${item.wallet}</strong><span>${item.amount}</span><em>${item.usd}</em></li>`)
    .join("");

  const allocationRows = allocations
    .map(
      (item) => `<li><label>${item.bucket} <span>${item.percent}%</span></label><div class="bar"><i style="width:${item.percent}%"></i></div></li>`
    )
    .join("");

  const predictionRows = predictions
    .map(
      (item) => `<li><strong>${item.symbol}</strong><span class="${item.signal.toLowerCase()}">${item.signal}</span><em>${item.confidence}%</em><small>${item.target}</small></li>`
    )
    .join("");

  const onChainRows = onChain
    .map((item) => `<li><span>${item.metric}</span><strong>${item.value}</strong><em>${item.trend}</em></li>`)
    .join("");

  res.type("html").send(`<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>ΩMEGA PRIME ∆ 2026 Terminal</title>
  </head>
  <body>
    <main>
      <header>
        <div>
          <span>ΩMEGA PRIME ∆ 2026</span>
          <span>${terminalSnapshot.status}</span>
          <div>${terminalSnapshot.price} · ${terminalSnapshot.dayPnl} · ${terminalSnapshot.latency}</div>
        </div>
      </header>
      <section>${chartTiles}</section>
      <section>${topPanelTiles}</section>
      <section>
        <article>
          <h4>Wallets & Fund Management</h4>
          <ul>${walletRows}</ul>
          <ul>${allocationRows}</ul>
        </article>
        <article>
          <h4>AI Predictions</h4>
          <ul>${predictionRows}</ul>
          <div>Social sentiment score: ${sentiment.score}/100</div>
        </article>
        <article>
          <h4>On-Chain Analytics</h4>
          <ul>${onChainRows}</ul>
        </article>
      </section>
    </main>
  </body>
</html>`);
});

app.listen(port, () => {
  console.log(`dashboard listening on ${port}`);
});
