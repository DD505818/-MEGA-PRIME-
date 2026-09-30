// Package modelock is the canonical interpreter of the ΩMEGA PRIME Δ
// PAPER/LIVE runtime contract.
//
// PAPER remains the default operational posture. LIVE is a separately gated
// mode and is valid only when every release gate is explicit. Any ambiguous,
// contradictory, missing, or malformed combination resolves to no-trade.
package modelock

import (
	"fmt"
	"log"
	"os"
	"strings"
)

type Mode string

const (
	ModePaper Mode = "paper"
	ModeLive  Mode = "live"
)

// LiveCapable documents that the codebase contains a LIVE-capable execution
// path. It does NOT authorize a deployment; ResolveMode still requires all
// runtime certification gates and execution-service adds a signed release
// authorization before touching a broker.
const LiveCapable = true

// ResolveMode returns the one unambiguous runtime mode.
//
// PAPER requires:
//   PAPER_MODE=true
//   LIVE_TRADING_ENABLED != true
//   TRADING_MODE unset or PAPER
//
// LIVE requires all of:
//   PAPER_MODE != true
//   LIVE_TRADING_ENABLED=true
//   TRADING_MODE=LIVE
//   BROKER_CERTIFIED=true
//   FAILURE_TESTS_PASSED=true
//
// Everything else fails closed.
func ResolveMode() (Mode, error) {
	paper := isTrue(os.Getenv("PAPER_MODE"))
	live := isTrue(os.Getenv("LIVE_TRADING_ENABLED"))
	trading := strings.ToUpper(strings.TrimSpace(os.Getenv("TRADING_MODE")))

	if paper && !live && (trading == "" || trading == "PAPER") {
		return ModePaper, nil
	}
	if !paper && live && trading == "LIVE" &&
		isTrue(os.Getenv("BROKER_CERTIFIED")) &&
		isTrue(os.Getenv("FAILURE_TESTS_PASSED")) {
		return ModeLive, nil
	}

	return "", fmt.Errorf(
		"modelock: invalid mode contract (PAPER_MODE=%q LIVE_TRADING_ENABLED=%q TRADING_MODE=%q BROKER_CERTIFIED=%q FAILURE_TESTS_PASSED=%q)",
		os.Getenv("PAPER_MODE"),
		os.Getenv("LIVE_TRADING_ENABLED"),
		os.Getenv("TRADING_MODE"),
		os.Getenv("BROKER_CERTIFIED"),
		os.Getenv("FAILURE_TESTS_PASSED"),
	)
}

func IsPaper() bool {
	mode, err := ResolveMode()
	return err == nil && mode == ModePaper
}

func IsLive() bool {
	mode, err := ResolveMode()
	return err == nil && mode == ModeLive
}

// RequireMode terminates startup unless one canonical mode can be proven.
func RequireMode(service string) Mode {
	mode, err := ResolveMode()
	if err != nil {
		log.Fatalf("modelock: %s refusing to start: %v", service, err)
	}
	log.Printf("modelock: %s started in %s mode", service, strings.ToUpper(string(mode)))
	return mode
}

// RequirePaper is retained for explicitly PAPER-only processes/tests.
func RequirePaper(service string) {
	mode := RequireMode(service)
	if mode != ModePaper {
		log.Fatalf("modelock: %s is PAPER-only; resolved mode=%s", service, mode)
	}
}

// AssertMode re-resolves environment state on every sensitive path, detecting
// runtime flag mutation and refusing if it no longer matches startup mode.
func AssertMode(expected Mode) error {
	mode, err := ResolveMode()
	if err != nil {
		return err
	}
	if mode != expected {
		return fmt.Errorf("modelock: runtime mode changed from %s to %s", expected, mode)
	}
	return nil
}

func AssertPaper() error {
	return AssertMode(ModePaper)
}

func isTrue(v string) bool {
	return strings.EqualFold(strings.TrimSpace(v), "true")
}
