const express = require("express");

const app = express();
const port = 3001;

const actions = [
  "Start/stop strategies",
  "Change risk parameters",
  "Deploy new AI models",
  "View strategy PnL",
  "Restart services"
];

const automations = [
  { name: "Auto-hedge", state: "Enabled" },
  { name: "Circuit breaker", state: "Armed" },
  { name: "Model rollback", state: "Ready" },
  { name: "Order throttle", state: "Adaptive" }
];

app.get("/api/actions", (_req, res) => {
  res.json({ app: "control-panel", actions, automations });
});

app.get("/", (_req, res) => {
  const actionRows = actions
    .map((action, index) => `<li><span>${index + 1}.</span>${action}</li>`)
    .join("");

  const automationRows = automations
    .map((rule) => `<div class="badge"><strong>${rule.name}</strong><em>${rule.state}</em></div>`)
    .join("");

  res.type("html").send(`<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>ΩMEGA PRIME Control Panel</title>
    <style>
      :root {
        color-scheme: dark;
        --bg: #070b14;
        --panel: #11192c;
        --panel-2: #0e1626;
        --edge: rgba(124, 211, 255, 0.25);
        --text: #eef5ff;
        --muted: #91a4c2;
        --accent: #7cd3ff;
      }
      * { box-sizing: border-box; }
      body {
        margin: 0;
        font-family: Inter, system-ui, sans-serif;
        background:
          radial-gradient(circle at 0% 100%, rgba(124, 211, 255, 0.2), transparent 40%),
          radial-gradient(circle at 100% 0%, rgba(98, 255, 206, 0.16), transparent 35%),
          var(--bg);
        color: var(--text);
        min-height: 100vh;
        padding: 2rem;
      }
      main {
        max-width: 960px;
        margin: 0 auto;
        display: grid;
        gap: 1rem;
      }
      section {
        background: linear-gradient(165deg, var(--panel), var(--panel-2));
        border: 1px solid var(--edge);
        border-radius: 20px;
        padding: 1.2rem 1.3rem;
      }
      h1, h2 { margin: 0; }
      p { color: var(--muted); }
      ul {
        margin: 1rem 0 0;
        padding: 0;
        list-style: none;
        display: grid;
        gap: 0.6rem;
      }
      li {
        border: 1px solid rgba(145, 164, 194, 0.2);
        border-radius: 12px;
        padding: 0.6rem 0.8rem;
        background: rgba(9, 14, 25, 0.55);
        display: flex;
        gap: 0.6rem;
      }
      li span {
        color: var(--accent);
        min-width: 1.4rem;
      }
      .automations {
        display: flex;
        flex-wrap: wrap;
        gap: 0.65rem;
        margin-top: 1rem;
      }
      .badge {
        display: flex;
        flex-direction: column;
        gap: 0.2rem;
        border-radius: 12px;
        border: 1px solid rgba(124, 211, 255, 0.3);
        padding: 0.6rem 0.8rem;
        min-width: 170px;
        background: rgba(124, 211, 255, 0.08);
      }
      em {
        color: #9ad6ff;
        font-style: normal;
        font-size: 0.85rem;
      }
    </style>
  </head>
  <body>
    <main>
      <section>
        <h1>ΩMEGA PRIME Control Panel</h1>
        <p>Live operational center for deployment, risk governance, and service orchestration.</p>
      </section>
      <section>
        <h2>Primary operations</h2>
        <ul>${actionRows}</ul>
      </section>
      <section>
        <h2>Safety automations</h2>
        <div class="automations">${automationRows}</div>
      </section>
    </main>
  </body>
</html>`);
});

app.listen(port, () => {
  console.log(`control panel listening on ${port}`);
});
