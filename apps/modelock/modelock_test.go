package modelock

import (
	"os"
	"testing"
)

func TestIsPaperMatrix(t *testing.T) {
	cases := []struct {
		name     string
		paper    string // "" means unset
		liveFlag string // "" means unset
		want     bool
	}{
		{"paper true, live unset", "true", "", true},
		{"paper TRUE mixed case, live false", "TrUe", "false", true},
		{"paper padded whitespace", "  true  ", "", true},
		{"paper unset", "", "", false},
		{"paper empty, live unset", "", "", false},
		{"paper false", "false", "", false},
		{"paper 1", "1", "", false},
		{"paper yes", "yes", "", false},
		{"paper live-word", "live", "", false},
		{"live flag true vetoes paper true", "true", "true", false},
		{"live flag TRUE vetoes", "true", "TRUE", false},
		{"live flag true alone", "", "true", false},
		{"live flag garbage is not a veto, paper still governs", "true", "maybe", true},
		{"paper false, live flag false", "false", "false", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.paper == "" {
				os.Unsetenv("PAPER_MODE")
			} else {
				t.Setenv("PAPER_MODE", tc.paper)
			}
			if tc.liveFlag == "" {
				os.Unsetenv("LIVE_TRADING_ENABLED")
			} else {
				t.Setenv("LIVE_TRADING_ENABLED", tc.liveFlag)
			}
			if got := IsPaper(); got != tc.want {
				t.Fatalf("IsPaper() = %v, want %v (PAPER_MODE=%q LIVE_TRADING_ENABLED=%q)",
					got, tc.want, tc.paper, tc.liveFlag)
			}
			if err := AssertPaper(); (err == nil) != tc.want {
				t.Fatalf("AssertPaper() err = %v, want paper=%v", err, tc.want)
			}
		})
	}
}

func TestLiveLockedConstant(t *testing.T) {
	if !LiveLocked {
		t.Fatal("LiveLocked must remain true until a certification program authorizes change")
	}
}

// TestAssertPaperErrorMessage ensures the refusal is explicit, not silent.
func TestAssertPaperErrorMessage(t *testing.T) {
	t.Setenv("PAPER_MODE", "false")
	t.Setenv("LIVE_TRADING_ENABLED", "")
	if err := AssertPaper(); err == nil {
		t.Fatal("expected refusal outside paper mode")
	} else if got := err.Error(); got == "" {
		t.Fatal("refusal must carry an explicit message")
	}
}
