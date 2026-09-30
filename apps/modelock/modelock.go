// Package modelock is the single canonical interpreter of the ΩMEGA PRIME Δ
// PAPER/LIVE mode contract.
//
// Governing constraint: LIVE IS LOCKED. There is no code path in this
// repository that can enable live broker submission, and there is no
// supported way to set LIVE mode until a separate certification program
// explicitly authorizes changing that constraint.
//
// Mode contract (environment):
//
//	PAPER_MODE=true             → paper trading (the only runnable mode)
//	LIVE_TRADING_ENABLED=true   → hard refusal, unconditionally
//	anything else, including unset or malformed → NOT paper → fail closed
//
// Every service in the trading path must call RequirePaper at startup, and
// every order-submission path must pass through AssertPaper. An ambiguous or
// missing mode resolves to no-trade, never to live.
package modelock

import (
	"fmt"
	"log"
	"os"
	"strings"
)

// LiveLocked documents the governing constraint in code: live trading cannot
// be enabled by configuration. Kept as a constant so any future attempt to
// introduce a live path must confront this symbol in review.
const LiveLocked = true

// IsPaper reports whether this process is configured for paper trading.
// Fail-closed: only an explicit PAPER_MODE=true (case-insensitive, trimmed)
// counts as paper, and LIVE_TRADING_ENABLED=true vetoes everything.
func IsPaper() bool {
	if isTrue(os.Getenv("LIVE_TRADING_ENABLED")) {
		return false
	}
	return isTrue(os.Getenv("PAPER_MODE"))
}

// RequirePaper terminates the process unless it is configured for paper
// trading. Call once at startup, before any consumer, server, or worker
// starts. This is intentionally fatal: a service that cannot prove it is in
// paper mode must not run at all.
func RequirePaper(service string) {
	if !IsPaper() {
		log.Fatalf(
			"modelock: %s refusing to start: PAPER_MODE must be \"true\" and "+
				"LIVE_TRADING_ENABLED must not be \"true\" (LIVE is locked)",
			service,
		)
	}
	log.Printf("modelock: %s started in PAPER mode (LIVE locked)", service)
}

// AssertPaper returns nil only when the process is in paper trading mode.
// Call on every order-submission path (worker, API, retry, replay) so a
// mode change after startup can never leak an order toward a live venue.
func AssertPaper() error {
	if !IsPaper() {
		return fmt.Errorf("modelock: refusing order path outside PAPER_MODE=true (LIVE is locked)")
	}
	return nil
}

func isTrue(v string) bool {
	return strings.EqualFold(strings.TrimSpace(v), "true")
}
