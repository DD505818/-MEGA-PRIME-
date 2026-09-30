package modelock

import (
	"os"
	"testing"
)

func clearModeEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"PAPER_MODE", "LIVE_TRADING_ENABLED", "TRADING_MODE",
		"BROKER_CERTIFIED", "FAILURE_TESTS_PASSED",
	} {
		t.Setenv(k, "")
	}
}

func TestResolveModePaper(t *testing.T) {
	clearModeEnv(t)
	t.Setenv("PAPER_MODE", "true")
	mode, err := ResolveMode()
	if err != nil || mode != ModePaper {
		t.Fatalf("mode=%q err=%v, want paper", mode, err)
	}
	if !IsPaper() || IsLive() {
		t.Fatal("paper predicates disagree")
	}
	if err := AssertMode(ModePaper); err != nil {
		t.Fatalf("AssertMode(paper): %v", err)
	}
}

func TestResolveModeLiveRequiresEveryGate(t *testing.T) {
	clearModeEnv(t)
	t.Setenv("LIVE_TRADING_ENABLED", "true")
	t.Setenv("TRADING_MODE", "LIVE")
	t.Setenv("BROKER_CERTIFIED", "true")
	t.Setenv("FAILURE_TESTS_PASSED", "true")
	mode, err := ResolveMode()
	if err != nil || mode != ModeLive {
		t.Fatalf("mode=%q err=%v, want live", mode, err)
	}
	if !IsLive() || IsPaper() {
		t.Fatal("live predicates disagree")
	}
}

func TestResolveModeLiveMissingAnyGateFails(t *testing.T) {
	keys := []string{"LIVE_TRADING_ENABLED", "TRADING_MODE", "BROKER_CERTIFIED", "FAILURE_TESTS_PASSED"}
	for _, missing := range keys {
		t.Run(missing, func(t *testing.T) {
			clearModeEnv(t)
			t.Setenv("LIVE_TRADING_ENABLED", "true")
			t.Setenv("TRADING_MODE", "LIVE")
			t.Setenv("BROKER_CERTIFIED", "true")
			t.Setenv("FAILURE_TESTS_PASSED", "true")
			t.Setenv(missing, "")
			if _, err := ResolveMode(); err == nil {
				t.Fatalf("missing %s must fail closed", missing)
			}
		})
	}
}

func TestResolveModeContradictionsFail(t *testing.T) {
	cases := []map[string]string{
		{"PAPER_MODE": "true", "LIVE_TRADING_ENABLED": "true", "TRADING_MODE": "LIVE", "BROKER_CERTIFIED": "true", "FAILURE_TESTS_PASSED": "true"},
		{"PAPER_MODE": "true", "LIVE_TRADING_ENABLED": "false", "TRADING_MODE": "LIVE"},
		{"PAPER_MODE": "false", "LIVE_TRADING_ENABLED": "false", "TRADING_MODE": "PAPER"},
		{"PAPER_MODE": "false", "LIVE_TRADING_ENABLED": "true", "TRADING_MODE": "PAPER", "BROKER_CERTIFIED": "true", "FAILURE_TESTS_PASSED": "true"},
	}
	for i, env := range cases {
		t.Run(string(rune('A'+i)), func(t *testing.T) {
			clearModeEnv(t)
			for k, v := range env {
				t.Setenv(k, v)
			}
			if _, err := ResolveMode(); err == nil {
				t.Fatalf("contradictory env unexpectedly resolved: %#v", env)
			}
		})
	}
}

func TestAssertModeDetectsRuntimeMutation(t *testing.T) {
	clearModeEnv(t)
	t.Setenv("PAPER_MODE", "true")
	if err := AssertMode(ModePaper); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PAPER_MODE", "false")
	t.Setenv("LIVE_TRADING_ENABLED", "true")
	t.Setenv("TRADING_MODE", "LIVE")
	t.Setenv("BROKER_CERTIFIED", "true")
	t.Setenv("FAILURE_TESTS_PASSED", "true")
	if err := AssertMode(ModePaper); err == nil {
		t.Fatal("runtime mode mutation must be refused")
	}
}

func TestLiveCapableConstant(t *testing.T) {
	if !LiveCapable {
		t.Fatal("Phase 5 requires LIVE-capable code with runtime gates")
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
