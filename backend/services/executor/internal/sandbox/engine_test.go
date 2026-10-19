package sandbox

import (
	"strings"
	"testing"
	"time"
)

func TestContainerLifetimeCoversEveryPhase(t *testing.T) {
	t.Parallel()

	req := Request{CompileTimeout: 20 * time.Second, RunTimeout: 10 * time.Second}
	got := containerLifetime(req)

	floor := req.CompileTimeout + req.RunTimeout + 2*sourceWriteTimeout
	if got <= floor {
		t.Errorf("containerLifetime = %v, want > %v (sum of phase timeouts)", got, floor)
	}
	// Bounded: a leaked container must stop within minutes, not a day.
	if got > 5*time.Minute {
		t.Errorf("containerLifetime = %v, want a few minutes at most", got)
	}
}

func TestTmpfsOptionsAreLockedDown(t *testing.T) {
	t.Parallel()

	for _, exec := range []bool{false, true} {
		opts := tmpfsOptions(64<<20, exec)
		for _, want := range []string{"nosuid", "nodev", "size=67108864"} {
			if !strings.Contains(opts, want) {
				t.Errorf("tmpfsOptions(exec=%v) = %q, missing %q", exec, opts, want)
			}
		}
		if !exec && !strings.Contains(opts, "noexec") {
			t.Errorf("tmpfsOptions(exec=false) = %q, want noexec", opts)
		}
	}
}
