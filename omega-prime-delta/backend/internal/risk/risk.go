package risk

import "fmt"

type Limits struct {
	MaxDailyLossPct float64
	MaxDrawdownPct  float64
	MaxOrderQty     float64
	MaxExposurePct  float64
}

type CheckInput struct {
	Equity      float64
	DailyLoss   float64
	DrawdownPct float64
	OrderQty    float64
	ExposurePct float64
}

type CheckResult struct {
	Approved bool
	Reasons  []string
}

func DefaultLimits() Limits {
	return Limits{
		MaxDailyLossPct: 0.02,
		MaxDrawdownPct:  0.10,
		MaxOrderQty:     100,
		MaxExposurePct:  0.25,
	}
}

func ValidateOrder(input CheckInput, limits Limits) CheckResult {
	reasons := make([]string, 0, 4)

	if input.Equity <= 0 {
		reasons = append(reasons, "equity must be positive")
	}
	if input.OrderQty <= 0 {
		reasons = append(reasons, "order quantity must be positive")
	}

	if input.Equity > 0 && -input.DailyLoss >= limits.MaxDailyLossPct*input.Equity {
		reasons = append(reasons, fmt.Sprintf("daily loss limit reached: %.4f", input.DailyLoss))
	}
	if input.DrawdownPct >= limits.MaxDrawdownPct {
		reasons = append(reasons, fmt.Sprintf("drawdown limit reached: %.4f", input.DrawdownPct))
	}
	if input.OrderQty > limits.MaxOrderQty {
		reasons = append(reasons, fmt.Sprintf("order quantity exceeds limit: %.4f", input.OrderQty))
	}
	if input.ExposurePct > limits.MaxExposurePct {
		reasons = append(reasons, fmt.Sprintf("exposure exceeds limit: %.4f", input.ExposurePct))
	}

	return CheckResult{Approved: len(reasons) == 0, Reasons: reasons}
}
