package strategies

type BoxTheory struct {
	PDH float64
	PDL float64
	Mid float64
}

func (b *BoxTheory) ComputeLevels(high, low float64) {
	b.PDH = high
	b.PDL = low
	b.Mid = (high + low) / 2
}

func (b *BoxTheory) CheckSweep(currentHigh, currentLow float64) (sweptHigh, sweptLow bool) {
	return currentHigh > b.PDH, currentLow < b.PDL
}

func (b *BoxTheory) CheckRejection(candleClose, candleHigh, candleLow float64, side string) bool {
	if side == "BUY" {
		return candleClose > b.PDL && (candleHigh-candleClose) > (candleClose-candleLow)
	}
	return candleClose < b.PDH && (candleClose-candleLow) > (candleHigh-candleClose)
}

func (b *BoxTheory) GenerateSignal(dailyHigh, dailyLow, intradayHigh, intradayLow, intradayClose float64) map[string]interface{} {
	b.ComputeLevels(dailyHigh, dailyLow)
	sweptHigh, sweptLow := b.CheckSweep(intradayHigh, intradayLow)
	if sweptLow && b.CheckRejection(intradayClose, intradayHigh, intradayLow, "BUY") {
		return map[string]interface{}{
			"direction": "BUY",
			"entry":     intradayClose,
			"stop":      b.PDL,
			"tp1":       b.Mid,
			"tp2":       b.PDH,
			"risk":      intradayClose - b.PDL,
			"reward1":   b.Mid - intradayClose,
			"reward2":   b.PDH - intradayClose,
		}
	}
	if sweptHigh && b.CheckRejection(intradayClose, intradayHigh, intradayLow, "SELL") {
		return map[string]interface{}{
			"direction": "SELL",
			"entry":     intradayClose,
			"stop":      b.PDH,
			"tp1":       b.Mid,
			"tp2":       b.PDL,
			"risk":      b.PDH - intradayClose,
			"reward1":   intradayClose - b.Mid,
			"reward2":   intradayClose - b.PDL,
		}
	}
	return nil
}
