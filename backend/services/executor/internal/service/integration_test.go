package service

// Integration tests exercising the full pipeline against a real Docker
// daemon and the arena-sandbox-* images.
//
// Opt-in: set ARENA_SANDBOX_TESTS=1 and build images first
// (task executor:images). Tests run sequentially on purpose — each spawns
// real containers under real resource limits, and the per-test leak check
// must observe an empty daemon.

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"

	executorv1 "github.com/caezu/arena/backend/pkg/pb/executor/v1"

	"github.com/caezu/arena/backend/services/executor/internal/config"
	"github.com/caezu/arena/backend/services/executor/internal/sandbox"
)

func newIntegrationService(t *testing.T) *Service {
	t.Helper()

	if os.Getenv("ARENA_SANDBOX_TESTS") != "1" {
		t.Skip("set ARENA_SANDBOX_TESTS=1 (and run 'task executor:images') to run sandbox integration tests")
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	engine, err := sandbox.NewEngine(ctx, cfg.DockerHost, discardLogger())
	if err != nil {
		t.Fatalf("connect to docker: %v", err)
	}
	t.Cleanup(func() {
		assertNoLeakedSandboxes(t)
		if err := engine.Close(); err != nil {
			t.Errorf("close engine: %v", err)
		}
	})

	return New(engine, cfg, discardLogger())
}

// assertNoLeakedSandboxes proves the auto-destroy invariant: after every
// test, zero arena sandbox containers may exist (running or stopped).
func assertNoLeakedSandboxes(t *testing.T) {
	t.Helper()

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Fatalf("docker client for leak check: %v", err)
	}
	defer func() {
		if err := cli.Close(); err != nil {
			t.Errorf("close leak-check client: %v", err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	leaked, err := cli.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: filters.NewArgs(filters.Arg("label", "arena.sandbox=1")),
	})
	if err != nil {
		t.Fatalf("list containers for leak check: %v", err)
	}
	if len(leaked) != 0 {
		t.Errorf("%d sandbox container(s) leaked after test", len(leaked))
	}
}

func execute(t *testing.T, svc *Service, req *executorv1.ExecuteRequest) *executorv1.ExecuteResponse {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	start := time.Now()
	resp, err := svc.Execute(ctx, req)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	t.Logf("%s: verdict=%s wall=%v run=%dms exit=%d",
		req.GetLanguage(), resp.GetVerdict(), time.Since(start).Round(time.Millisecond),
		resp.GetTimeUsedMs(), resp.GetExitCode())
	return resp
}

// --- verdict matrix -------------------------------------------------------

const (
	acceptedCPP = `#include <bits/stdc++.h>
using namespace std;
int main() { long long x; cin >> x; cout << x * 2 << "\n"; }
`
	acceptedPython = `x = int(input())
print(x * 2)
`
	acceptedGo = `package main

import "fmt"

func main() {
	var x int64
	fmt.Scan(&x)
	fmt.Println(x * 2)
}
`
)

func TestIntegrationAccepted(t *testing.T) {
	svc := newIntegrationService(t)

	tests := []struct {
		language executorv1.Language
		code     string
	}{
		{executorv1.Language_LANGUAGE_CPP, acceptedCPP},
		{executorv1.Language_LANGUAGE_PYTHON, acceptedPython},
		{executorv1.Language_LANGUAGE_GO, acceptedGo},
	}

	for _, tt := range tests {
		t.Run(tt.language.String(), func(t *testing.T) {
			resp := execute(t, svc, &executorv1.ExecuteRequest{
				Code:           tt.code,
				Language:       tt.language,
				Stdin:          "21\n",
				ExpectedOutput: "42",
			})
			if resp.GetVerdict() != executorv1.Verdict_VERDICT_ACCEPTED {
				t.Errorf("verdict = %v, want ACCEPTED (stderr: %q, compile: %q)",
					resp.GetVerdict(), resp.GetStderr(), resp.GetCompileOutput())
			}
		})
	}
}

func TestIntegrationWrongAnswer(t *testing.T) {
	svc := newIntegrationService(t)

	resp := execute(t, svc, &executorv1.ExecuteRequest{
		Code:           "print(41)",
		Language:       executorv1.Language_LANGUAGE_PYTHON,
		ExpectedOutput: "42",
	})
	if resp.GetVerdict() != executorv1.Verdict_VERDICT_WRONG_ANSWER {
		t.Errorf("verdict = %v, want WRONG_ANSWER", resp.GetVerdict())
	}
	if !strings.Contains(resp.GetStdout(), "41") {
		t.Errorf("stdout = %q, want the wrong output preserved for feedback", resp.GetStdout())
	}
}

func TestIntegrationCompilationError(t *testing.T) {
	svc := newIntegrationService(t)

	tests := []struct {
		language executorv1.Language
		code     string
	}{
		{executorv1.Language_LANGUAGE_CPP, "int main() { return 0 }"},
		{executorv1.Language_LANGUAGE_PYTHON, "def f(:\n    pass"},
		{executorv1.Language_LANGUAGE_GO, "package main\n\nfunc main() {"},
	}

	for _, tt := range tests {
		t.Run(tt.language.String(), func(t *testing.T) {
			resp := execute(t, svc, &executorv1.ExecuteRequest{
				Code:           tt.code,
				Language:       tt.language,
				ExpectedOutput: "0",
			})
			if resp.GetVerdict() != executorv1.Verdict_VERDICT_COMPILATION_ERROR {
				t.Errorf("verdict = %v, want COMPILATION_ERROR", resp.GetVerdict())
			}
			if resp.GetCompileOutput() == "" {
				t.Error("compile_output is empty, want compiler diagnostics")
			}
		})
	}
}

func TestIntegrationRuntimeError(t *testing.T) {
	svc := newIntegrationService(t)

	tests := []struct {
		language executorv1.Language
		code     string
	}{
		{executorv1.Language_LANGUAGE_CPP, "#include <cstdlib>\nint main() { std::abort(); }"},
		{executorv1.Language_LANGUAGE_PYTHON, "print(1 / 0)"},
		{executorv1.Language_LANGUAGE_GO, "package main\n\nfunc main() { panic(\"boom\") }"},
	}

	for _, tt := range tests {
		t.Run(tt.language.String(), func(t *testing.T) {
			resp := execute(t, svc, &executorv1.ExecuteRequest{
				Code:           tt.code,
				Language:       tt.language,
				ExpectedOutput: "0",
			})
			if resp.GetVerdict() != executorv1.Verdict_VERDICT_RUNTIME_ERROR {
				t.Errorf("verdict = %v, want RUNTIME_ERROR", resp.GetVerdict())
			}
			if resp.GetExitCode() == 0 {
				t.Error("exit_code = 0, want non-zero for a crash")
			}
		})
	}
}

func TestIntegrationTimeLimitExceeded(t *testing.T) {
	svc := newIntegrationService(t)

	start := time.Now()
	resp := execute(t, svc, &executorv1.ExecuteRequest{
		Code:           "while True:\n    pass",
		Language:       executorv1.Language_LANGUAGE_PYTHON,
		ExpectedOutput: "0",
		TimeLimitMs:    1000,
	})
	elapsed := time.Since(start)

	if resp.GetVerdict() != executorv1.Verdict_VERDICT_TIME_LIMIT_EXCEEDED {
		t.Errorf("verdict = %v, want TIME_LIMIT_EXCEEDED", resp.GetVerdict())
	}
	// The kill must land near the limit — an unkilled loop would run forever.
	if elapsed > 30*time.Second {
		t.Errorf("infinite loop took %v end-to-end, kill did not land near the 1s limit", elapsed)
	}
}

func TestIntegrationMemoryLimitExceeded(t *testing.T) {
	svc := newIntegrationService(t)

	// Allocate and touch memory in 16 MB strides until the 128 MB cgroup
	// limit kills the process.
	code := `#include <cstring>
#include <vector>
int main() {
    std::vector<char*> chunks;
    for (int i = 0; i < 100; i++) {
        char* p = new char[16 << 20];
        std::memset(p, 1, 16 << 20);
        chunks.push_back(p);
    }
    return 0;
}
`
	resp := execute(t, svc, &executorv1.ExecuteRequest{
		Code:           code,
		Language:       executorv1.Language_LANGUAGE_CPP,
		ExpectedOutput: "0",
	})
	if resp.GetVerdict() != executorv1.Verdict_VERDICT_MEMORY_LIMIT_EXCEEDED {
		t.Errorf("verdict = %v, want MEMORY_LIMIT_EXCEEDED (exit=%d stderr=%q)",
			resp.GetVerdict(), resp.GetExitCode(), resp.GetStderr())
	}
}

// --- malicious submissions ------------------------------------------------

// Each malicious test is written so that the *contained* outcome prints a
// marker and exits 0 — the ACCEPTED verdict is then positive proof the
// sandbox control worked.

func TestIntegrationMaliciousNetworkAccess(t *testing.T) {
	svc := newIntegrationService(t)

	code := `import socket
try:
    socket.create_connection(("1.1.1.1", 80), timeout=1)
    print("connected")
except OSError:
    print("blocked")
`
	resp := execute(t, svc, &executorv1.ExecuteRequest{
		Code:           code,
		Language:       executorv1.Language_LANGUAGE_PYTHON,
		ExpectedOutput: "blocked",
		TimeLimitMs:    5000,
	})
	if resp.GetVerdict() != executorv1.Verdict_VERDICT_ACCEPTED {
		t.Errorf("network was not blocked: verdict=%v stdout=%q", resp.GetVerdict(), resp.GetStdout())
	}
}

func TestIntegrationMaliciousForkBomb(t *testing.T) {
	svc := newIntegrationService(t)

	code := `import os
for i in range(10000):
    try:
        pid = os.fork()
    except OSError:
        print("contained")
        break
    if pid == 0:
        os._exit(0)
else:
    print("not contained")
`
	resp := execute(t, svc, &executorv1.ExecuteRequest{
		Code:           code,
		Language:       executorv1.Language_LANGUAGE_PYTHON,
		ExpectedOutput: "contained",
		TimeLimitMs:    5000,
	})
	if resp.GetVerdict() != executorv1.Verdict_VERDICT_ACCEPTED {
		t.Errorf("fork bomb was not contained: verdict=%v stdout=%q stderr=%q",
			resp.GetVerdict(), resp.GetStdout(), resp.GetStderr())
	}
}

func TestIntegrationMaliciousFilesystemWrite(t *testing.T) {
	svc := newIntegrationService(t)

	code := `results = []
for path in ("/etc/pwned", "/usr/pwned", "/pwned", "/etc/passwd"):
    try:
        with open(path, "w") as f:
            f.write("pwned")
        results.append("wrote " + path)
    except OSError:
        results.append("denied")
print(";".join(results))
`
	resp := execute(t, svc, &executorv1.ExecuteRequest{
		Code:           code,
		Language:       executorv1.Language_LANGUAGE_PYTHON,
		ExpectedOutput: "denied;denied;denied;denied",
		TimeLimitMs:    5000,
	})
	if resp.GetVerdict() != executorv1.Verdict_VERDICT_ACCEPTED {
		t.Errorf("filesystem write was not blocked: verdict=%v stdout=%q",
			resp.GetVerdict(), resp.GetStdout())
	}
}

func TestIntegrationMaliciousPrivilegeEscalation(t *testing.T) {
	svc := newIntegrationService(t)

	code := `import os
try:
    os.setuid(0)
    print("escalated to uid", os.getuid())
except PermissionError:
    print("blocked")
`
	resp := execute(t, svc, &executorv1.ExecuteRequest{
		Code:           code,
		Language:       executorv1.Language_LANGUAGE_PYTHON,
		ExpectedOutput: "blocked",
		TimeLimitMs:    5000,
	})
	if resp.GetVerdict() != executorv1.Verdict_VERDICT_ACCEPTED {
		t.Errorf("privilege escalation was not blocked: verdict=%v stdout=%q",
			resp.GetVerdict(), resp.GetStdout())
	}
}

func TestIntegrationMaliciousOutputFlood(t *testing.T) {
	svc := newIntegrationService(t)

	// ~16 MB of stdout against a 1 MiB capture cap.
	code := `for _ in range(16384):
    print("A" * 1023)
`
	resp := execute(t, svc, &executorv1.ExecuteRequest{
		Code:           code,
		Language:       executorv1.Language_LANGUAGE_PYTHON,
		ExpectedOutput: "0",
		TimeLimitMs:    10000,
	})

	if got, capBytes := len(resp.GetStdout()), 1<<20; got > capBytes {
		t.Errorf("captured stdout = %d bytes, want <= %d (flood must be truncated)", got, capBytes)
	}
	if v := resp.GetVerdict(); v != executorv1.Verdict_VERDICT_WRONG_ANSWER {
		t.Errorf("verdict = %v, want WRONG_ANSWER (truncated flood cannot match)", v)
	}
}

// --- concurrency ----------------------------------------------------------

func TestIntegrationConcurrentSubmissions(t *testing.T) {
	svc := newIntegrationService(t)

	const n = 4
	var wg sync.WaitGroup
	verdicts := make([]executorv1.Verdict, n)
	errs := make([]error, n)

	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()

			resp, err := svc.Execute(ctx, &executorv1.ExecuteRequest{
				Code:           fmt.Sprintf("x = int(input())\nprint(x * %d)", i+1),
				Language:       executorv1.Language_LANGUAGE_PYTHON,
				Stdin:          "10\n",
				ExpectedOutput: fmt.Sprintf("%d", 10*(i+1)),
			})
			if err != nil {
				errs[i] = err
				return
			}
			verdicts[i] = resp.GetVerdict()
		}()
	}
	wg.Wait()

	for i := range n {
		if errs[i] != nil {
			t.Errorf("submission %d: %v", i, errs[i])
			continue
		}
		if verdicts[i] != executorv1.Verdict_VERDICT_ACCEPTED {
			t.Errorf("submission %d: verdict = %v, want ACCEPTED", i, verdicts[i])
		}
	}
}
