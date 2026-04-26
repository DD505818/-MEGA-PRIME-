import numpy as np
from enum import Enum

class BoxState(Enum):
    IDLE = 0
    SWEEP = 1
    REENTRY = 2
    REJECTION = 3

class BoxTheory:
    def __init__(self):
        self.state = BoxState.IDLE
        self.cumulative_high = -np.inf
        self.cumulative_low = np.inf
        self.pdh = self.pdl = None
        self.atr = 0.0
        self.sweep_dir = None
        self.last_date = None
        self.name = "BoxTheory"

    def generate_signal(self, daily_df, intraday_df):
        if daily_df is None or intraday_df is None:
            return None
        current_date = daily_df.index[-1].date()
        if self.last_date is None or current_date != self.last_date:
            self.last_date = current_date
            self.state = BoxState.IDLE
            self.cumulative_high = -np.inf
            self.cumulative_low = np.inf
        self.pdh = daily_df['high'].iloc[-2]
        self.pdl = daily_df['low'].iloc[-2]
        self.atr = self._calc_atr(daily_df)
        for idx, bar in intraday_df.iterrows():
            self.cumulative_high = max(self.cumulative_high, bar['high'])
            self.cumulative_low = min(self.cumulative_low, bar['low'])
            if self.state == BoxState.IDLE:
                if bar['close'] > self.pdh and (self.cumulative_high - self.pdh) > 0.5 * self.atr:
                    self.sweep_dir = "bullish"
                    self.state = BoxState.SWEEP
                elif bar['close'] < self.pdl and (self.pdl - self.cumulative_low) > 0.5 * self.atr:
                    self.sweep_dir = "bearish"
                    self.state = BoxState.SWEEP
            elif self.state == BoxState.SWEEP:
                if self.sweep_dir == "bullish" and bar['close'] < self.pdh and bar['close'] > self.pdl:
                    self.state = BoxState.REENTRY
                elif self.sweep_dir == "bearish" and bar['close'] > self.pdl and bar['close'] < self.pdh:
                    self.state = BoxState.REENTRY
            elif self.state == BoxState.REENTRY:
                if self._check_rejection(bar):
                    if self.atr > 0.005 * daily_df['close'].iloc[-1]:
                        entry = bar['close']
                        stop = self.pdl if self.sweep_dir == "bullish" else self.pdh
                        target = self.pdh if self.sweep_dir == "bullish" else self.pdl
                        self.state = BoxState.IDLE
                        self.cumulative_high = -np.inf
                        self.cumulative_low = np.inf
                        return {
                            "side": "BUY" if self.sweep_dir=="bullish" else "SELL",
                            "entry": entry, "stop": stop, "target": target,
                            "confidence": 0.7
                        }
        self.cumulative_high = -np.inf
        self.cumulative_low = np.inf
        return None

    def _check_rejection(self, bar):
        if self.sweep_dir == "bullish":
            return bar['close'] < bar['open'] and bar['close'] < self.pdh
        return bar['close'] > bar['open'] and bar['close'] > self.pdl

    def _calc_atr(self, df, period=14):
        high = df['high']; low = df['low']; close = df['close']
        tr1 = high - low
        tr2 = np.abs(high - close.shift())
        tr3 = np.abs(low - close.shift())
        tr = np.maximum(np.maximum(tr1, tr2), tr3)
        return tr.rolling(period).mean().iloc[-1]
