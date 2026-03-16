const express = require("express");

const app = express();
const port = 3000;

const widgets = [
  "Multi-chart trading terminal",
  "Portfolio metrics",
  "Strategy performance",
  "AI agent monitoring",
  "Risk metrics",
  "Execution logs",
  "Orderbook heatmap",
  "Wallet & fund management",
  "AI predictions",
  "Social sentiment",
  "On-chain analytics"
];

const stats = [
  { label: "Connected exchanges", value: "5", detail: "Binance · Kraken · Coinbase · Bybit · Alpaca" },
  { label: "Models online", value: "12", detail: "PPO, DQN, regime detectors" },
  { label: "Latency budget", value: "41ms", detail: "Smart router p95 observed" },
  { label: "Risk posture", value: "Green", detail: "All constraints within thresholds" }
];

const wallets = [
  { name: "MetaMask", balance: "3.2 ETH", usd: "$8,420" },
  { name: "WalletConnect", balance: "12 SOL", usd: "$1,740" },
  { name: "Ledger", balance: "0.5 BTC", usd: "$32,115" },
  { name: "Trezor", balance: "45 AVAX", usd: "$1,553" },
  { name: "Phantom", balance: "150 DOT", usd: "$1,095" }
];

const predictions = [
  { symbol: "BTC", signal: "BULLISH", confidence: 95, target: "$69,200" },
  { symbol: "ETH", signal: "BULLISH", confidence: 89, target: "$3,750" },
  { symbol: "SOL", signal: "BULLISH", confidence: 78, target: "$210" },
  { symbol: "AVAX", signal: "BEARISH", confidence: 65, target: "$31" }
];

const onChain = [
  { metric: "Exchange Inflow", value: "-8.2%", trend: "↓" },
  { metric: "Exchange Outflow", value: "+11.4%", trend: "↑" },
  { metric: "Whale Wallets", value: "1,342", trend: "↑" },
  { metric: "Active Addresses", value: "2.8M", trend: "↑" },
  { metric: "Hashrate", value: "612 EH/s", trend: "↑" },
  { metric: "Funding Rate", value: "0.011%", trend: "↓" }
];

app.get("/api/widgets", (_req, res) => {
  res.json({ app: "dashboard", widgets, stats, wallets, predictions, onChain });
});

app.get("/", (_req, res) => {
  const walletRows = wallets
    .map((wallet) => `<li><strong>${wallet.name}</strong><span>${wallet.balance} · ${wallet.usd}</span></li>`)
    .join("");

  const predictionRows = predictions
    .map(
      (row) => `<li><strong>${row.symbol}</strong><span class="${row.signal.toLowerCase()}">${row.signal}</span><em>${row.confidence}%</em><small>${row.target}</small></li>`
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
        --bg: #03060f;
        --panel: #0d1427;
        --panel2: #0a1120;
        --line: rgba(135, 156, 211, 0.24);
        --text: #e9efff;
        --muted: #95a5d2;
        --good: #3ee795;
        --danger: #ff4f6b;
        --warn: #ffb340;
        --chip: #152242;
      }
      * { box-sizing: border-box; }
      body {
        margin: 0;
        font-family: Inter, system-ui, sans-serif;
        color: var(--text);
        background: radial-gradient(circle at 0% 0%, rgba(124, 92, 255, 0.2), transparent 34%), var(--bg);
      }
      .wrap { max-width: 1440px; margin: 0 auto; padding: 1rem; }
      .topbar {
        display: grid;
        grid-template-columns: 1.5fr auto auto auto auto auto;
        gap: .6rem;
        align-items: center;
        background: linear-gradient(130deg, #121d35, #0b1223);
        border: 1px solid var(--line);
        border-radius: 16px;
        padding: .85rem 1rem;
      }
      .brand { font-weight: 700; }
      .pill { font-size: .85rem; color: var(--muted); }
      .btn {
        border: 1px solid var(--line);
        background: #101a31;
        color: var(--text);
        border-radius: 10px;
        padding: .55rem .75rem;
        cursor: pointer;
        position: relative;
        overflow: hidden;
      }
      .btn:hover { transform: translateY(-1px); }
      .btn.start.active { box-shadow: 0 0 20px rgba(62, 231, 149, 0.6); animation: pulse-green 1.2s infinite; }
      .btn.stop.active { box-shadow: 0 0 20px rgba(255,79,107,.6); animation: pulse-red 1.2s infinite; }
      .btn.emergency { border-color: rgba(255,79,107,.5); }
      .btn.emergency.active { animation: flash 0.5s infinite; }
      .btn.emergency:hover { background: #ff274f; }
      @keyframes pulse-green { 0%,100%{opacity:1}50%{opacity:.72} }
      @keyframes pulse-red { 0%,100%{opacity:1}50%{opacity:.72} }
      @keyframes flash { 0%,100%{background:#6f1426}50%{background:#ff335a} }
      .charts {
        margin-top: .9rem;
        display: grid;
        grid-template-columns: repeat(2, 1fr);
        gap: .75rem;
      }
      .chart {
        background: linear-gradient(150deg, var(--panel), var(--panel2));
        border: 1px solid var(--line);
        border-radius: 14px;
        min-height: 160px;
        padding: .8rem;
        position: relative;
      }
      .chart .indicator {
        position: absolute; top: 0; left: 0; right: 0; height: 4px;
        background: transparent;
      }
      .chart.trading .indicator { background: linear-gradient(90deg, #3ee795, #7bffbf); animation: pulse-green 1s infinite; }
      .panel-grid {
        margin-top: .9rem;
        display: grid;
        grid-template-columns: repeat(5, 1fr);
        gap: .7rem;
      }
      .panel {
        background: linear-gradient(155deg, var(--panel), #070d19);
        border: 1px solid var(--line);
        border-radius: 14px;
        padding: .65rem;
        min-height: 180px;
      }
      .panel h3 { margin: 0 0 .5rem; font-size: .9rem; }
      .muted { color: var(--muted); font-size: .8rem; }
      .wallets ul, .pred ul, .chain ul { list-style: none; margin: 0; padding: 0; display: grid; gap: .35rem; }
      .wallets li, .pred li, .chain li {
        display: grid;
        grid-template-columns: 1fr auto;
        gap: .5rem;
        border: 1px solid rgba(148,165,205,.2);
        border-radius: 10px;
        padding: .35rem .45rem;
        font-size: .78rem;
      }
      .pred li { grid-template-columns: 35px 1fr auto auto; align-items: center; }
      .bullish { color: var(--good); }
      .bearish { color: var(--danger); }
      .alloc { margin-top: .45rem; display: grid; gap: .25rem; }
      .bar { background: #0f1c36; border-radius: 6px; overflow: hidden; height: 8px; }
      .bar > span { display: block; height: 100%; background: linear-gradient(90deg, #58a6ff, #7c5cff); }
      .controls input { width: 100%; }
      .controls label { font-size: .75rem; color: var(--muted); display: block; margin-top: .4rem; }
      .quick { display: flex; flex-wrap: wrap; gap: .35rem; }
      .quick button { padding: .35rem .45rem; font-size: .72rem; }
      .sentiment .meter {
        margin-top: .6rem;
        height: 10px;
        background: #111c33;
        border-radius: 8px;
        overflow: hidden;
        display: grid;
        grid-template-columns: 25% 30% 45%;
      }
      .sentiment .meter span:nth-child(1) { background: #ff4f6b; }
      .sentiment .meter span:nth-child(2) { background: #ffb340; }
      .sentiment .meter span:nth-child(3) { background: #3ee795; }
      .notif {
        position: fixed; right: 1rem; bottom: 1rem; display: grid; gap: .45rem; z-index: 20;
      }
      .toast {
        background: #0d1932; border: 1px solid var(--line); padding: .55rem .75rem; border-radius: 10px;
        transform: translateX(120%); animation: slide-in .25s forwards;
      }
      @keyframes slide-in { to { transform: translateX(0); } }
      .ripple {
        position: absolute; border-radius: 50%; transform: scale(0); background: rgba(255,255,255,.35);
        animation: ripple .6s linear;
      }
      @keyframes ripple { to { transform: scale(4); opacity: 0; } }
      @media (max-width: 1100px) {
        .panel-grid { grid-template-columns: repeat(2, 1fr); }
        .topbar { grid-template-columns: 1fr 1fr 1fr; }
      }
    </style>
  </head>
  <body>
    <div class="wrap">
      <header class="topbar">
        <div class="brand">ΩMEGA PRIME ∆ | LIVE | $42,310 | +$820</div>
        <div class="pill">Latency: 41ms</div>
        <button class="btn start" data-action="Start Trading">Start</button>
        <button class="btn stop" data-action="Stop Trading">Stop</button>
        <button class="btn" data-action="Pause Trading">Pause</button>
        <button class="btn emergency" data-action="Emergency Stop">EMRG</button>
      </header>

      <section class="charts" id="charts">
        <article class="chart"><div class="indicator"></div><h3>BTCUSDT</h3><p class="muted">Live orderflow + microstructure</p></article>
        <article class="chart"><div class="indicator"></div><h3>ETHUSDT</h3><p class="muted">Adaptive momentum channel</p></article>
        <article class="chart"><div class="indicator"></div><h3>SOLUSDT</h3><p class="muted">Volatility expansion tracking</p></article>
        <article class="chart"><div class="indicator"></div><h3>NASDAQ</h3><p class="muted">Cross-asset correlation monitor</p></article>
      </section>

      <section class="panel-grid">
        <article class="panel"><h3>Market Watch</h3><p class="muted">BTC +2.1% · ETH +1.3% · DXY -0.2%</p></article>
        <article class="panel"><h3>Orderbook</h3><p class="muted">Bid/Ask imbalance: +8.7%</p></article>
        <article class="panel"><h3>Liquidity Heatmap</h3><p class="muted">High density near BTC 66k</p></article>
        <article class="panel"><h3>Portfolio & Risk</h3><p class="muted">VaR: 1.9% · DD: 3.1% · Exposure: 22%</p></article>
        <article class="panel wallets">
          <h3>Wallets & Fund Mgmt</h3>
          <p><strong>Total AUM:</strong> $2.4M</p>
          <ul>${walletRows}</ul>
          <div class="alloc">
            <small>Spot 45%</small><div class="bar"><span style="width:45%"></span></div>
            <small>Futures 30%</small><div class="bar"><span style="width:30%"></span></div>
            <small>Staking 15%</small><div class="bar"><span style="width:15%"></span></div>
            <small>Yield 10%</small><div class="bar"><span style="width:10%"></span></div>
          </div>
          <div class="quick" style="margin-top:.45rem;"><button class="btn" data-action="Deposit">Deposit</button><button class="btn" data-action="Withdraw">Withdraw</button><button class="btn" data-action="Rebalance">Rebalance</button></div>
        </article>

        <article class="panel pred">
          <h3>AI Predictions</h3>
          <ul>${predictionRows}</ul>
        </article>
        <article class="panel sentiment">
          <h3>Social Sentiment</h3>
          <p class="muted">Community mood: <strong>72/100</strong></p>
          <div class="meter"><span></span><span></span><span></span></div>
          <p class="muted">Bear 25% · Neutral 30% · Bull 45%</p>
        </article>
        <article class="panel chain">
          <h3>On-chain Analytics</h3>
          <ul>${onChainRows}</ul>
        </article>
        <article class="panel"><h3>Execution Router</h3><p class="muted">Best venue: Binance · Failover ready</p></article>
        <article class="panel controls">
          <h3>Portfolio Controls</h3>
          <div class="quick">
            <button class="btn" data-action="Start All">Start All</button>
            <button class="btn" data-action="Stop All">Stop All</button>
            <button class="btn" data-action="Close All">Close All</button>
            <button class="btn" data-action="Hedge">Hedge</button>
          </div>
          <label for="risk">Risk Level: <strong id="riskValue">45%</strong></label>
          <input id="risk" type="range" min="0" max="100" value="45" />
          <label for="lev">Leverage: <strong id="levValue">8x</strong></label>
          <input id="lev" type="range" min="1" max="20" value="8" />
          <button class="btn" style="margin-top:.5rem;width:100%;" data-action="Initialize System">Initialize System</button>
        </article>
      </section>
    </div>

    <aside class="notif" id="notif"></aside>

    <script>
      const charts = [...document.querySelectorAll('.chart')];
      const notif = document.getElementById('notif');
      const risk = document.getElementById('risk');
      const lev = document.getElementById('lev');
      const riskValue = document.getElementById('riskValue');
      const levValue = document.getElementById('levValue');

      function toast(msg, isError = false) {
        const div = document.createElement('div');
        div.className = 'toast';
        div.style.borderColor = isError ? 'rgba(255,79,107,.5)' : 'rgba(62,231,149,.45)';
        div.textContent = (isError ? '⚠ ' : '✅ ') + msg;
        notif.appendChild(div);
        setTimeout(() => div.remove(), 3000);
      }

      function ripple(event) {
        const button = event.currentTarget;
        const circle = document.createElement('span');
        const size = Math.max(button.clientWidth, button.clientHeight);
        const rect = button.getBoundingClientRect();
        circle.style.width = circle.style.height = size + 'px';
        circle.style.left = (event.clientX - rect.left - size / 2) + 'px';
        circle.style.top = (event.clientY - rect.top - size / 2) + 'px';
        circle.className = 'ripple';
        button.appendChild(circle);
        setTimeout(() => circle.remove(), 650);
      }

      document.querySelectorAll('.btn').forEach((btn) => {
        btn.addEventListener('click', (e) => {
          ripple(e);
          const action = btn.dataset.action || btn.textContent.trim();
          if (btn.classList.contains('start')) {
            btn.classList.toggle('active');
            charts.forEach((chart) => chart.classList.toggle('trading', btn.classList.contains('active')));
          }
          if (btn.classList.contains('stop')) btn.classList.toggle('active');
          if (btn.classList.contains('emergency')) btn.classList.toggle('active');
          toast(action + ' triggered', btn.classList.contains('emergency'));
        });
      });

      risk.addEventListener('input', () => { riskValue.textContent = risk.value + '%'; });
      lev.addEventListener('input', () => { levValue.textContent = lev.value + 'x'; });
    </script>
  </body>
</html>`);
});

app.listen(port, () => {
  console.log(`dashboard listening on ${port}`);
});
