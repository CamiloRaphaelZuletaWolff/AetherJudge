package judge

import (
	"testing"

	executorv1 "github.com/caezu/arena/backend/pkg/pb/executor/v1"
)

func TestOutputsMatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		expected string
		actual   string
		want     bool
	}{
		{"identical", "42\n", "42\n", true},
		{"missing trailing newline", "42\n", "42", true},
		{"extra trailing newlines", "42", "42\n\n\n", true},
		{"trailing spaces on line", "1 2 3", "1 2 3   ", true},
		{"trailing tabs and CR", "ok", "ok\t\r", true},
		{"crlf output", "a\nb", "a\r\nb\r\n", true},
		{"multiline equal", "1\n2\n3", "1\n2\n3\n", true},
		{"empty both", "", "\n", true},
		{"wrong value", "42", "43", false},
		{"leading space significant", "42", " 42", false},
		{"interior blank line significant", "a\n\nb", "a\nb", false},
		{"case sensitive", "Yes", "yes", false},
		{"missing line", "1\n2", "1", false},
		{"extra interior content", "1\n2", "1\nx\n2", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := OutputsMatch(tt.expected, tt.actual); got != tt.want {
				t.Errorf("OutputsMatch(%q, %q) = %v, want %v", tt.expected, tt.actual, got, tt.want)
			}
		})
	}
}

func TestVerdict(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		result   Result
		expected string
		want     executorv1.Verdict
	}{
		{
			name:     "accepted",
			result:   Result{ExitCode: 0, Stdout: "42\n"},
			expected: "42",
			want:     executorv1.Verdict_VERDICT_ACCEPTED,
		},
		{
			name:     "wrong answer",
			result:   Result{ExitCode: 0, Stdout: "43"},
			expected: "42",
			want:     executorv1.Verdict_VERDICT_WRONG_ANSWER,
		},
		{
			name:     "runtime error",
			result:   Result{ExitCode: 1, Stdout: "42"},
			expected: "42",
			want:     executorv1.Verdict_VERDICT_RUNTIME_ERROR,
		},
		{
			name:     "timeout wins over everything",
			result:   Result{ExitCode: 137, TimedOut: true, OOMKilled: true},
			expected: "42",
			want:     executorv1.Verdict_VERDICT_TIME_LIMIT_EXCEEDED,
		},
		{
			name:     "oom flag",
			result:   Result{ExitCode: 1, OOMKilled: true},
			expected: "42",
			want:     executorv1.Verdict_VERDICT_MEMORY_LIMIT_EXCEEDED,
		},
		{
			name:     "sigkill without timeout treated as oom",
			result:   Result{ExitCode: 137},
			expected: "42",
			want:     executorv1.Verdict_VERDICT_MEMORY_LIMIT_EXCEEDED,
		},
		{
			name:     "correct output but nonzero exit is runtime error",
			result:   Result{ExitCode: 2, Stdout: "42"},
			expected: "42",
			want:     executorv1.Verdict_VERDICT_RUNTIME_ERROR,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := Verdict(tt.result, tt.expected); got != tt.want {
				t.Errorf("Verdict(%+v, %q) = %v, want %v", tt.result, tt.expected, got, tt.want)
			}
		})
	}
}
