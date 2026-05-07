package execution

import "errors"

var (
	ErrRiskRejected           = errors.New("risk_rejected")
	ErrRiskServiceUnreachable = errors.New("risk_service_unreachable")
)
