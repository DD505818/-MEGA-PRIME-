const express = require("express");
const app = express();
const widgets = [
  "multi-chart trading terminal",
  "portfolio metrics",
  "strategy performance",
  "AI agent monitoring",
  "risk metrics",
  "execution logs",
  "orderbook heatmap"
];
app.get("/", (_req, res) => res.json({ app: "dashboard", widgets }));
app.listen(3000, () => console.log("dashboard listening on 3000"));
