const express = require("express");
const app = express();
const actions = [
  "start/stop strategies",
  "change risk parameters",
  "deploy new AI models",
  "view strategy PnL",
  "restart services"
];
app.get("/", (_req, res) => res.json({ app: "control-panel", actions }));
app.listen(3001, () => console.log("control panel listening on 3001"));
