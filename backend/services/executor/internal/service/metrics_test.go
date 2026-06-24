package service

import (
	"testing"

	executorv1 "github.com/caezu/arena/backend/pkg/pb/executor/v1"
)

func TestVerdictLabelMatchesPlatformVocabulary(t *testing.T) {
	t.Parallel()

	want := map[executorv1.Verdict]string{
		executorv1.Verdict_VERDICT_ACCEPTED:              "accepted",
		executorv1.Verdict_VERDICT_WRONG_ANSWER:          "wrong_answer",
		executorv1.Verdict_VERDICT_RUNTIME_ERROR:         "runtime_error",
		executorv1.Verdict_VERDICT_COMPILATION_ERROR:     "compilation_error",
		executorv1.Verdict_VERDICT_TIME_LIMIT_EXCEEDED:   "time_limit_exceeded",
		executorv1.Verdict_VERDICT_MEMORY_LIMIT_EXCEEDED: "memory_limit_exceeded",
		executorv1.Verdict_VERDICT_INTERNAL_ERROR:        "internal_error",
	}
	for verdict, label := range want {
		if got := verdictLabel(verdict); got != label {
			t.Errorf("verdictLabel(%v) = %q, want %q (gateway and executor metrics must share one vocabulary)", verdict, got, label)
		}
	}
}
