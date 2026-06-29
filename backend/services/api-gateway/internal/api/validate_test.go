package api

import (
	"strings"
	"testing"
	"time"
)

func TestValidateUsername(t *testing.T) {
	t.Parallel()

	valid := []string{"abc", "alice_99", "A_B_C", strings.Repeat("a", 32)}
	for _, u := range valid {
		if err := validateUsername(u); err != nil {
			t.Errorf("validateUsername(%q) = %v, want nil", u, err)
		}
	}

	invalid := []string{"", "ab", strings.Repeat("a", 33), "has space", "héllo", "a-b", "x@y"}
	for _, u := range invalid {
		if err := validateUsername(u); err == nil {
			t.Errorf("validateUsername(%q) = nil, want error", u)
		}
	}
}

func TestValidateEmail(t *testing.T) {
	t.Parallel()

	for _, e := range []string{"a@b.co", "user+tag@example.com"} {
		if err := validateEmail(e); err != nil {
			t.Errorf("validateEmail(%q) = %v, want nil", e, err)
		}
	}
	for _, e := range []string{"", "no-at-sign", "@start", "end@", strings.Repeat("a", 250) + "@x.co"} {
		if err := validateEmail(e); err == nil {
			t.Errorf("validateEmail(%q) = nil, want error", e)
		}
	}
}

func TestValidatePassword(t *testing.T) {
	t.Parallel()

	if err := validatePassword("longenough"); err != nil {
		t.Errorf("validatePassword(valid) = %v", err)
	}
	for _, p := range []string{"short", strings.Repeat("a", 73)} {
		if err := validatePassword(p); err == nil {
			t.Errorf("validatePassword(%q) = nil, want error", p)
		}
	}
}

func TestValidateLanguageCodeStdin(t *testing.T) {
	t.Parallel()

	for _, l := range []string{"cpp", "python", "go"} {
		if err := validateLanguage(l); err != nil {
			t.Errorf("validateLanguage(%q) = %v", l, err)
		}
	}
	if err := validateLanguage("rust"); err == nil {
		t.Error("validateLanguage(rust) = nil, want error")
	}

	if err := validateCode("print(1)"); err != nil {
		t.Errorf("validateCode(valid) = %v", err)
	}
	if err := validateCode("   \n\t "); err == nil {
		t.Error("validateCode(whitespace) = nil, want error")
	}
	if err := validateCode(strings.Repeat("a", maxCodeBytes+1)); err == nil {
		t.Error("validateCode(oversize) = nil, want error")
	}

	if err := validateStdin(strings.Repeat("a", maxStdinBytes+1)); err == nil {
		t.Error("validateStdin(oversize) = nil, want error")
	}
	if err := validateStdin(""); err != nil {
		t.Errorf("validateStdin(empty) = %v", err)
	}
}

func TestValidateSlug(t *testing.T) {
	t.Parallel()

	for _, s := range []string{"abc", "weekly-cup", "round-2", strings.Repeat("a", 64)} {
		if err := validateSlug(s); err != nil {
			t.Errorf("validateSlug(%q) = %v, want nil", s, err)
		}
	}
	for _, s := range []string{"", "ab", "-leading", "Upper", "has space", "under_score", strings.Repeat("a", 65)} {
		if err := validateSlug(s); err == nil {
			t.Errorf("validateSlug(%q) = nil, want error", s)
		}
	}
}

func TestValidateContestTimesAndTitle(t *testing.T) {
	t.Parallel()

	now := time.Now()
	if err := validateContestTimes(now, now.Add(time.Hour)); err != nil {
		t.Errorf("validateContestTimes(valid) = %v", err)
	}
	if err := validateContestTimes(now, now); err == nil {
		t.Error("validateContestTimes(equal) = nil, want error")
	}
	if err := validateContestTimes(now, now.Add(-time.Hour)); err == nil {
		t.Error("validateContestTimes(end before start) = nil, want error")
	}

	if err := validateContestTitle("Weekly Cup"); err != nil {
		t.Errorf("validateContestTitle(valid) = %v", err)
	}
	for _, ti := range []string{"", "   ", strings.Repeat("a", 201)} {
		if err := validateContestTitle(ti); err == nil {
			t.Errorf("validateContestTitle(%q) = nil, want error", ti)
		}
	}
}

func TestValidateProblemFields(t *testing.T) {
	t.Parallel()

	if err := validateStatement("# Problem\n\nDo the thing."); err != nil {
		t.Errorf("validateStatement(valid) = %v", err)
	}
	for _, s := range []string{"", "   \n ", strings.Repeat("a", maxStatementBytes+1)} {
		if err := validateStatement(s); err == nil {
			t.Errorf("validateStatement(invalid len %d) = nil, want error", len(s))
		}
	}

	for _, ms := range []int{100, 2000, 10000} {
		if err := validateTimeLimit(ms); err != nil {
			t.Errorf("validateTimeLimit(%d) = %v", ms, err)
		}
	}
	for _, ms := range []int{99, 10001, 0, -1} {
		if err := validateTimeLimit(ms); err == nil {
			t.Errorf("validateTimeLimit(%d) = nil, want error", ms)
		}
	}

	for _, mb := range []int{16, 128, 512} {
		if err := validateMemoryLimit(mb); err != nil {
			t.Errorf("validateMemoryLimit(%d) = %v", mb, err)
		}
	}
	for _, mb := range []int{15, 513, 0} {
		if err := validateMemoryLimit(mb); err == nil {
			t.Errorf("validateMemoryLimit(%d) = nil, want error", mb)
		}
	}

	if err := validateTestCaseIO("1 2", "3"); err != nil {
		t.Errorf("validateTestCaseIO(valid) = %v", err)
	}
	if err := validateTestCaseIO("", ""); err != nil {
		t.Errorf("validateTestCaseIO(empty) = %v, want nil (empty allowed)", err)
	}
	if err := validateTestCaseIO(strings.Repeat("a", maxTestCaseFieldBytes+1), "x"); err == nil {
		t.Error("validateTestCaseIO(oversize stdin) = nil, want error")
	}
}
