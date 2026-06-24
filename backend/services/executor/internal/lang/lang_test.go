package lang

import (
	"testing"

	executorv1 "github.com/caezu/arena/backend/pkg/pb/executor/v1"
)

func TestForLanguageSupported(t *testing.T) {
	t.Parallel()

	for _, l := range []executorv1.Language{
		executorv1.Language_LANGUAGE_CPP,
		executorv1.Language_LANGUAGE_PYTHON,
		executorv1.Language_LANGUAGE_GO,
	} {
		spec, err := ForLanguage(l, "latest")
		if err != nil {
			t.Errorf("ForLanguage(%v): %v", l, err)
			continue
		}

		if spec.Name == "" || spec.Image == "" || spec.SourceFile == "" {
			t.Errorf("ForLanguage(%v): incomplete spec %+v", l, spec)
		}
		if len(spec.CompileCmd) == 0 {
			t.Errorf("ForLanguage(%v): missing compile command", l)
		}
		if len(spec.RunCmd) == 0 {
			t.Errorf("ForLanguage(%v): missing run command", l)
		}
		if spec.TmpfsSizeBytes <= 0 {
			t.Errorf("ForLanguage(%v): tmpfs size = %d, want > 0", l, spec.TmpfsSizeBytes)
		}
	}
}

func TestForLanguageUsesImageTag(t *testing.T) {
	t.Parallel()

	spec, err := ForLanguage(executorv1.Language_LANGUAGE_PYTHON, "v1.2.3")
	if err != nil {
		t.Fatalf("ForLanguage: %v", err)
	}
	if want := "arena-sandbox-python:v1.2.3"; spec.Image != want {
		t.Errorf("Image = %q, want %q", spec.Image, want)
	}
}

func TestForLanguageRejectsUnknown(t *testing.T) {
	t.Parallel()

	if _, err := ForLanguage(executorv1.Language_LANGUAGE_UNSPECIFIED, "latest"); err == nil {
		t.Error("ForLanguage(UNSPECIFIED) returned nil error, want error")
	}
	if _, err := ForLanguage(executorv1.Language(999), "latest"); err == nil {
		t.Error("ForLanguage(999) returned nil error, want error")
	}
}
