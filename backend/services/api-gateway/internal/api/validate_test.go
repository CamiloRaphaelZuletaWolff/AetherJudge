package api

import (
	"strings"
	"testing"
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
