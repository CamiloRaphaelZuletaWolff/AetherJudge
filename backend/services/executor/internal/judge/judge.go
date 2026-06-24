// Package judge turns raw execution observations into competitive-programming
// verdicts. It is pure logic with no Docker or gRPC dependencies.
package judge

import (
	"strings"

	executorv1 "github.com/caezu/arena/backend/pkg/pb/executor/v1"
)

// Result captures what the sandbox observed about the run phase.
type Result struct {
	ExitCode  int
	TimedOut  bool
	OOMKilled bool
	Stdout    string
}

// sigkillExit is 128+SIGKILL. When the process was SIGKILLed but the run did
// not hit the wall-clock limit, the kernel OOM killer is the only killer in
// the sandbox — used as a fallback when the daemon's OOMKilled flag is not
// reported (cgroup accounting differs across kernels/VMs).
const sigkillExit = 137

// Verdict judges one run result against the expected output.
//
// Precedence: timeout beats everything (an OOM flag can coexist with a kill
// we issued); then memory; then crashes; only a clean exit is compared.
func Verdict(r Result, expectedOutput string) executorv1.Verdict {
	switch {
	case r.TimedOut:
		return executorv1.Verdict_VERDICT_TIME_LIMIT_EXCEEDED
	case r.OOMKilled, r.ExitCode == sigkillExit:
		return executorv1.Verdict_VERDICT_MEMORY_LIMIT_EXCEEDED
	case r.ExitCode != 0:
		return executorv1.Verdict_VERDICT_RUNTIME_ERROR
	case OutputsMatch(expectedOutput, r.Stdout):
		return executorv1.Verdict_VERDICT_ACCEPTED
	default:
		return executorv1.Verdict_VERDICT_WRONG_ANSWER
	}
}

// OutputsMatch compares program output against the expected output using
// standard judge semantics: trailing whitespace on each line is ignored, as
// are trailing blank lines. Leading whitespace and interior blank lines are
// significant.
func OutputsMatch(expected, actual string) bool {
	return canonicalize(expected) == canonicalize(actual)
}

func canonicalize(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t\r")
	}

	end := len(lines)
	for end > 0 && lines[end-1] == "" {
		end--
	}

	return strings.Join(lines[:end], "\n")
}
