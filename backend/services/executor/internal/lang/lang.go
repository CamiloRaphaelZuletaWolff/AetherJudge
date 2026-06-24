// Package lang defines the per-language execution specifications: which
// sandbox image to use, where the source file lives, and the exact commands
// for each pipeline phase. It is pure data — no Docker dependencies — so the
// table is trivially unit-testable.
package lang

import (
	"fmt"

	executorv1 "github.com/caezu/arena/backend/pkg/pb/executor/v1"
)

// Spec describes how one language is compiled and run inside its sandbox.
type Spec struct {
	// Name is the short identifier used in logs and container labels.
	Name string
	// Image is the full sandbox image reference (name:tag).
	Image string
	// SourceFile is the absolute path the submission is written to in /box.
	SourceFile string
	// CompileCmd transforms the source into something runnable. For Python it
	// is a syntax check (py_compile) so syntax errors surface as
	// COMPILATION_ERROR, matching judge conventions.
	CompileCmd []string
	// CleanupCmd runs after a successful compile, before resource limits are
	// tightened. Used to drop build caches whose tmpfs pages would otherwise
	// count against the run-phase memory limit. Nil means no cleanup.
	CleanupCmd []string
	// RunCmd executes the compiled submission.
	RunCmd []string
	// TmpfsSizeBytes caps the writable /box tmpfs.
	TmpfsSizeBytes int64
	// TmpfsExec mounts /box with exec permission. Only languages that must
	// run a compiled binary out of /box need it; interpreted languages keep
	// the noexec default as defense in depth against dropped binaries.
	TmpfsExec bool
}

const (
	mib = 1 << 20

	imagePrefix = "arena-sandbox-"
)

// ForLanguage returns the execution spec for l, with sandbox images resolved
// at the given tag. It returns an error for unknown or unspecified languages.
func ForLanguage(l executorv1.Language, imageTag string) (Spec, error) {
	switch l {
	case executorv1.Language_LANGUAGE_CPP:
		return Spec{
			Name:           "cpp",
			Image:          imagePrefix + "cpp:" + imageTag,
			SourceFile:     "/box/main.cpp",
			CompileCmd:     []string{"g++", "-O2", "-std=c++20", "-o", "/box/prog", "/box/main.cpp"},
			RunCmd:         []string{"/box/prog"},
			TmpfsSizeBytes: 128 * mib,
			TmpfsExec:      true,
		}, nil
	case executorv1.Language_LANGUAGE_PYTHON:
		return Spec{
			Name:           "python",
			Image:          imagePrefix + "python:" + imageTag,
			SourceFile:     "/box/main.py",
			CompileCmd:     []string{"python", "-m", "py_compile", "/box/main.py"},
			RunCmd:         []string{"python", "/box/main.py"},
			TmpfsSizeBytes: 64 * mib,
		}, nil
	case executorv1.Language_LANGUAGE_GO:
		return Spec{
			Name:       "go",
			Image:      imagePrefix + "go:" + imageTag,
			SourceFile: "/box/main.go",
			// The pre-warmed stdlib cache (see go.Dockerfile) is copied onto
			// the tmpfs first: without it a build cold-compiles the standard
			// library (~73 s measured) instead of just the submission.
			CompileCmd: []string{"sh", "-c", "cp -R /opt/gocache /box/.gocache && go build -o /box/prog /box/main.go"},
			// The build cache lives on the tmpfs (GOCACHE=/box/.gocache, set
			// in the image); its pages are charged to the memory cgroup, so
			// it must be wiped before the limit shrinks to run size.
			CleanupCmd:     []string{"rm", "-rf", "/box/.gocache", "/box/.gopath"},
			RunCmd:         []string{"/box/prog"},
			TmpfsSizeBytes: 768 * mib,
			TmpfsExec:      true,
		}, nil
	default:
		return Spec{}, fmt.Errorf("lang: unsupported language %q", l.String())
	}
}
