package judge

import (
	"strconv"
	"strings"
	"testing"
)

// buildOutput returns an n-line output that mimics a real submission's stdout
// (one integer per line) with the trailing-whitespace noise the judge must
// canonicalize away.
func buildOutput(n int, trailingSpaces bool) string {
	var b strings.Builder
	for i := range n {
		b.WriteString(strconv.Itoa(i))
		if trailingSpaces {
			b.WriteString("   ")
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// BenchmarkOutputsMatch measures the output comparison on the matching path
// (the common case: every line must be canonicalized and compared).
func BenchmarkOutputsMatch(b *testing.B) {
	expected := buildOutput(1000, false)
	actual := buildOutput(1000, true) // same content, trailing whitespace differs

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if !OutputsMatch(expected, actual) {
			b.Fatal("expected a match")
		}
	}
}

// BenchmarkVerdict measures the full accepted-path verdict (the hot path,
// since every test case that passes runs the comparison).
func BenchmarkVerdict(b *testing.B) {
	expected := buildOutput(1000, false)
	r := Result{ExitCode: 0, Stdout: buildOutput(1000, true)}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = Verdict(r, expected)
	}
}
