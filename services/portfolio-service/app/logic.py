def allocate_capital(strategies):
    scored = []
    for s in strategies:
        score = s["sharpe"] * 0.5 - s["drawdown"] * 0.3 + s["regime_fit"] * 0.2
        scored.append((s["name"], max(score, 0)))
    total = sum(v for _, v in scored) or 1
    return {k: v / total for k, v in scored}
