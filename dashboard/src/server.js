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
  res.json({
    app: "dashboard",
    terminalSnapshot,
    charts,
    topPanels,
    walletBalances,
    allocations,
    predictions,
    sentiment,
    onChain
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
      (row) => `<li><strong>${row.symbol}</strong><span class="${row.signal === "BULLISH" ? "good" : "warn"}">${row.signal}</span><em>${row.confidence}%</em><b>${row.target}</b></li>`
    )
    .join("");

  const onChainRows = onChain
    .map((row) => `<li><span>${row.metric}</span><strong>${row.value}</strong><em>${row.trend}</em></li>`)
    .join("");

  res.type("html").send(`<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>ΩMEGA PRIME ∆ 2026 Terminal</title>
    <style>
      :root {
        --bg:#050a14;
        --panel:#0f1828;
        --panel2:#121e33;
        --line:#263753;
        --text:#e8f2ff;
        --muted:#9fb0cc;
        --green:#4df2b2;
        --red:#ff617d;
        --blue:#6dbbff;
      }
      *{box-sizing:border-box}
      body{margin:0;font-family:Inter,system-ui,sans-serif;background:radial-gradient(circle at 20% 5%, #17233b, transparent 40%),var(--bg);color:var(--text);padding:1rem}
      .terminal{max-width:1400px;margin:0 auto;display:grid;gap:0.85rem}
      .header{display:flex;justify-content:space-between;align-items:center;gap:.8rem;padding:.9rem 1rem;border:1px solid var(--line);border-radius:14px;background:linear-gradient(160deg,var(--panel),#0a1323)}
      .brand{font-weight:700;letter-spacing:.04em}
      .live{color:var(--green);font-weight:700;margin-left:.5rem}
      .metrics{display:flex;gap:.75rem;flex-wrap:wrap;color:var(--muted)}
      .controls{display:flex;gap:.45rem;flex-wrap:wrap}
      .btn{position:relative;border:1px solid #355075;background:#10213b;color:#d9e8ff;padding:.45rem .8rem;border-radius:999px;cursor:pointer;overflow:hidden;transition:.2s ease}
      .btn:hover{transform:translateY(-1px)}
      .btn.start.active{box-shadow:0 0 20px rgba(77,242,178,.55);border-color:rgba(77,242,178,.7)}
      .btn.stop.active{box-shadow:0 0 20px rgba(255,97,125,.5);border-color:rgba(255,97,125,.7)}
      .btn.emergency{animation:flash 1.2s infinite alternate;border-color:#ff617d}
      .btn.emergency:hover{background:#ff234f;color:#fff}
      .btn::after{content:"";position:absolute;inset:0;background:radial-gradient(circle,#fff6,transparent 55%);opacity:0;transition:opacity .3s}
      .btn:active::after{opacity:1}
      .chart-grid{display:grid;grid-template-columns:repeat(2,minmax(220px,1fr));gap:.75rem}
      .chart-card{min-height:120px;border:1px solid var(--line);border-radius:12px;background:linear-gradient(165deg,var(--panel2),#0b1527);padding:.75rem;display:flex;flex-direction:column;justify-content:space-between}
      .chart-card h3{margin:0;font-size:.95rem;color:#d9e9ff}
      .live-line{height:6px;border-radius:999px;background:linear-gradient(90deg,transparent,#34f39f,transparent);animation:pulse 1.6s infinite}
      .grid-six{display:grid;grid-template-columns:repeat(6,minmax(180px,1fr));gap:.65rem}
      .panel{border:1px solid var(--line);border-radius:12px;background:#0d1728;padding:.65rem}
      .kicker{margin:0;font-size:.7rem;text-transform:uppercase;color:var(--muted)}
      .panel h4{margin:.35rem 0 .2rem 0}
      .panel p{margin:0;color:var(--muted);font-size:.85rem}
      .bottom{display:grid;grid-template-columns:2fr 1fr 1fr;gap:.7rem}
      .section{border:1px solid var(--line);border-radius:12px;background:#0d1728;padding:.75rem}
      .section h4{margin:0 0 .6rem 0}
      ul{margin:0;padding:0;list-style:none;display:grid;gap:.45rem}
      .wallet li,.chain li,.pred li{display:grid;align-items:center;gap:.5rem;font-size:.84rem;border:1px solid #21314b;border-radius:10px;padding:.45rem .55rem;background:#0a1220}
      .wallet li{grid-template-columns:1.2fr 1fr .8fr}
      .pred li{grid-template-columns:.5fr 1fr .7fr .8fr}
      .chain li{grid-template-columns:1.2fr .8fr .2fr}
      .good{color:var(--green)} .warn{color:#ffc27a}
      .alloc label{display:flex;justify-content:space-between;color:var(--muted);font-size:.8rem}
      .bar{height:8px;border-radius:999px;background:#1a2740;margin-top:.3rem;overflow:hidden}
      .bar i{display:block;height:100%;background:linear-gradient(90deg,#5f9dff,#4df2b2)}
      .quick{display:flex;gap:.45rem;flex-wrap:wrap;margin-bottom:.6rem}
      .note{position:fixed;right:1rem;bottom:1rem;background:#102742;border:1px solid #2f4f76;padding:.7rem .85rem;border-radius:10px;animation:slideIn .4s ease}
      .sentiment{margin-top:.7rem}
      .sentiment .row{display:flex;height:10px;border-radius:999px;overflow:hidden}
      .bear{background:#ff617d;width:${sentiment.bear}%}.neutral{background:#7687a5;width:${sentiment.neutral}%}.bull{background:#4df2b2;width:${sentiment.bull}%}
      .actions{display:flex;gap:.5rem;margin-top:.6rem}
      .actions button{border:1px solid #355075;background:#0f223d;color:#cfe2ff;padding:.38rem .65rem;border-radius:8px}
      input[type='range']{width:100%}
      @keyframes pulse{0%{opacity:.35}50%{opacity:1}100%{opacity:.35}}
      @keyframes flash{0%{box-shadow:0 0 0 rgba(255,97,125,.15)}100%{box-shadow:0 0 18px rgba(255,97,125,.7)}}
      @keyframes slideIn{from{transform:translateY(14px);opacity:0}to{transform:translateY(0);opacity:1}}
      @media (max-width:1200px){.grid-six{grid-template-columns:repeat(3,minmax(180px,1fr))}.bottom{grid-template-columns:1fr}.header{flex-direction:column;align-items:flex-start}}
    </style>
  </head>
  <body>
    <main class="terminal">
      <header class="header">
        <div>
          <span class="brand">ΩMEGA PRIME ∆ 2026</span>
          <span class="live">${terminalSnapshot.status}</span>
          <div class="metrics">${terminalSnapshot.price} · ${terminalSnapshot.dayPnl} · ${terminalSnapshot.latency}</div>
        </div>
        <div class="controls">
          <button class="btn start active">Start</button>
          <button class="btn stop">Stop</button>
          <button class="btn">Pause</button>
          <button class="btn emergency">Emergency Stop</button>
        </div>
      </header>
      <section class="chart-grid">${chartTiles}</section>
      <section class="grid-six">${topPanelTiles}</section>
      <section class="bottom">
        <article class="section wallet">
          <h4>Wallets & Fund Management</h4>
          <ul>${walletRows}</ul>
          <ul class="alloc">${allocationRows}</ul>
          <div class="actions"><button>Deposit</button><button>Withdraw</button><button>Rebalance</button></div>
        </article>
        <article class="section pred">
          <h4>AI Predictions</h4>
          <ul>${predictionRows}</ul>
          <div class="sentiment">
            <small>Social sentiment score: ${sentiment.score}/100</small>
            <div class="row"><span class="bear"></span><span class="neutral"></span><span class="bull"></span></div>
          </div>
        </article>
        <article class="section chain">
          <h4>On-Chain Analytics</h4>
          <ul>${onChainRows}</ul>
          <h4 style="margin-top:.8rem">Portfolio Controls</h4>
          <div class="quick"><button class="btn">Start All</button><button class="btn">Stop All</button><button class="btn">Close All</button><button class="btn">Hedge</button></div>
          <small>Risk Level</small><input type="range" min="0" max="100" value="62" />
          <small>Leverage</small><input type="range" min="1" max="20" value="8" />
        </article>
      </section>
    </main>
    <div class="note">✅ Trading notification: System initialized.</div>
  </body>
</html>`);
});

app.listen(port, () => {
  console.log(`dashboard listening on ${port}`);
});
