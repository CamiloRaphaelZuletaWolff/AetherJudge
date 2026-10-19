// Package sandbox runs untrusted code inside locked-down ephemeral Docker
// containers. It is the only executor package that talks to the Docker
// daemon.
//
// Security invariants enforced on every container (see ADR-0005):
// no network, read-only rootfs, non-root user, all capabilities dropped,
// no-new-privileges, pids limit, memory/CPU quotas, and force-removal on
// every exit path. The only writable space is a size-capped tmpfs at /box.
package sandbox

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"

	"github.com/caezu/arena/backend/services/executor/internal/lang"
)

// sandboxLabel marks every container this engine creates, so leaked
// containers are findable (and integration tests can prove none leak).
const sandboxLabel = "arena.sandbox"

// sandboxUser matches the unprivileged user baked into the sandbox images.
const sandboxUser = "10001:10001"

// Engine executes submissions against a Docker daemon.
type Engine struct {
	cli *client.Client
	log *slog.Logger
}

// NewEngine connects to the Docker daemon and verifies it is reachable.
// host overrides the daemon endpoint; empty selects standard environment
// resolution (DOCKER_HOST, named pipe on Windows, Unix socket on Linux).
func NewEngine(ctx context.Context, host string, log *slog.Logger) (*Engine, error) {
	opts := []client.Opt{client.FromEnv, client.WithAPIVersionNegotiation()}
	if host != "" {
		opts = append(opts, client.WithHost(host))
	}

	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, fmt.Errorf("sandbox: create docker client: %w", err)
	}

	if _, err := cli.Ping(ctx); err != nil {
		closeErr := cli.Close()
		if closeErr != nil {
			log.Warn("close docker client after failed ping", "error", closeErr)
		}
		return nil, fmt.Errorf("sandbox: ping docker daemon (is Docker running?): %w", err)
	}

	return &Engine{cli: cli, log: log}, nil
}

// Close releases the Docker client.
func (e *Engine) Close() error {
	if err := e.cli.Close(); err != nil {
		return fmt.Errorf("sandbox: close docker client: %w", err)
	}
	return nil
}

// Resources bounds one phase of an execution.
type Resources struct {
	MemoryBytes int64
	NanoCPUs    int64
	PidsLimit   int64
}

// Request describes one complete sandboxed execution.
type Request struct {
	Spec  lang.Spec
	Code  string
	Stdin string

	// Compile bounds the compile phase (server policy, generous).
	Compile        Resources
	CompileTimeout time.Duration

	// Run bounds the run phase (strict, partly client-influenced but clamped
	// by the service layer before it gets here).
	Run        Resources
	RunTimeout time.Duration

	// OutputLimitBytes caps captured stdout/stderr, per stream.
	OutputLimitBytes int64
}

// Result reports what happened. CompileFailed short-circuits judging: when
// set, only CompileOutput is meaningful.
type Result struct {
	CompileFailed bool
	CompileOutput string

	ExitCode  int
	TimedOut  bool
	OOMKilled bool
	Stdout    string
	Stderr    string
	Duration  time.Duration
}

// compileOutputLimit caps captured compiler diagnostics. Server policy, not
// client-facing: enough for any real error message, small enough that a
// template-bomb cannot balloon memory.
const compileOutputLimit = 64 * 1024

// sandboxNofile caps open file descriptors per sandbox process (soft = hard,
// so user code cannot raise it). Compilers and runtimes need a few dozen.
const sandboxNofile = 1024

// lifetimeGrace pads the container's own self-destruct timer beyond the sum
// of every phase timeout (image start, exec setup, inspect round-trips).
const lifetimeGrace = 60 * time.Second

// sourceWriteTimeout bounds writing the source file into the container —
// pure I/O over the API socket, so seconds suffice even for max-size code.
const sourceWriteTimeout = 10 * time.Second

// Execute runs one submission through the full pipeline:
// create → write source → compile → drop caches → tighten limits → run.
// The container is force-removed on every path, including panics.
func (e *Engine) Execute(ctx context.Context, req Request) (Result, error) {
	language := req.Spec.Name

	createCtx, endCreate := startPhase(ctx, "create_container", language)
	id, err := e.createContainer(createCtx, req)
	if err != nil {
		endCreate(err)
		return Result{}, err
	}
	defer func() {
		// No span: removal runs after the request's trace is over (and must
		// happen even when ctx is dead); the histogram still sees it.
		start := time.Now()
		e.removeContainer(id)
		phaseDuration.WithLabelValues("remove_container", language).Observe(time.Since(start).Seconds())
	}()

	if err := e.cli.ContainerStart(createCtx, id, container.StartOptions{}); err != nil {
		err = fmt.Errorf("sandbox: start container: %w", err)
		endCreate(err)
		return Result{}, err
	}
	endCreate(nil)

	// Phase: deliver source over exec stdin (no mounts, no docker cp — the
	// rootfs stays read-only and nothing touches the host filesystem).
	writeCtx, endWrite := startPhase(ctx, "write_source", language)
	write, err := e.exec(writeCtx, id, execSpec{
		cmd:         []string{"sh", "-c", "cat > " + req.Spec.SourceFile},
		stdin:       req.Code,
		timeout:     sourceWriteTimeout,
		outputLimit: compileOutputLimit,
	})
	if err != nil {
		endWrite(err)
		return Result{}, err
	}
	if write.exitCode != 0 || write.timedOut {
		err := fmt.Errorf("sandbox: write source file: exit=%d timedOut=%v stderr=%q",
			write.exitCode, write.timedOut, write.stderr)
		endWrite(err)
		return Result{}, err
	}
	endWrite(nil)

	// Phase: compile (every language has one; Python's is a syntax check).
	compileCtx, endCompile := startPhase(ctx, "compile", language)
	compile, err := e.exec(compileCtx, id, execSpec{
		cmd:         req.Spec.CompileCmd,
		timeout:     req.CompileTimeout,
		outputLimit: compileOutputLimit,
	})
	endCompile(err)
	if err != nil {
		return Result{}, err
	}
	if compile.timedOut {
		return Result{CompileFailed: true, CompileOutput: "compilation exceeded the time limit"}, nil
	}
	if compile.exitCode != 0 {
		return Result{CompileFailed: true, CompileOutput: compile.stdout + compile.stderr}, nil
	}

	if req.Spec.CleanupCmd != nil {
		// Build caches live on the tmpfs and their pages count against the
		// memory cgroup — wipe them before the limit shrinks to run size.
		cleanupCtx, endCleanup := startPhase(ctx, "cleanup", language)
		cleanup, err := e.exec(cleanupCtx, id, execSpec{
			cmd:         req.Spec.CleanupCmd,
			timeout:     sourceWriteTimeout,
			outputLimit: compileOutputLimit,
		})
		if err != nil {
			endCleanup(err)
			return Result{}, err
		}
		if cleanup.exitCode != 0 || cleanup.timedOut {
			err := fmt.Errorf("sandbox: post-compile cleanup: exit=%d timedOut=%v stderr=%q",
				cleanup.exitCode, cleanup.timedOut, cleanup.stderr)
			endCleanup(err)
			return Result{}, err
		}
		endCleanup(nil)
	}

	// Phase: tighten the same container down to strict run limits.
	if err := e.updateResources(ctx, id, req.Run); err != nil {
		return Result{}, err
	}

	// Phase: run user code against the test case.
	runCtx, endRun := startPhase(ctx, "run", language)
	run, err := e.exec(runCtx, id, execSpec{
		cmd:         req.Spec.RunCmd,
		stdin:       req.Stdin,
		timeout:     req.RunTimeout,
		outputLimit: req.OutputLimitBytes,
	})
	endRun(err)
	if err != nil {
		return Result{}, err
	}

	return Result{
		ExitCode:  run.exitCode,
		TimedOut:  run.timedOut,
		OOMKilled: e.oomKilled(id),
		Stdout:    run.stdout,
		Stderr:    run.stderr,
		Duration:  run.duration,
	}, nil
}

func (e *Engine) createContainer(ctx context.Context, req Request) (string, error) {
	pids := req.Compile.PidsLimit

	cfg := &container.Config{
		Image: req.Spec.Image,
		// The container idles; the executor drives every phase via exec and
		// destroys it when done. Killing it is the timeout mechanism. The
		// idle process is bounded by the request's own phase budget, so a
		// container leaked by an executor crash stops by itself instead of
		// holding its memory/pids reservation for a day (ReapStale removes
		// the stopped husk later).
		Cmd:             []string{"sleep", strconv.Itoa(int(containerLifetime(req).Seconds()))},
		User:            sandboxUser,
		WorkingDir:      "/box",
		NetworkDisabled: true,
		Labels: map[string]string{
			sandboxLabel: "1",
			"arena.lang": req.Spec.Name,
		},
	}

	hostCfg := &container.HostConfig{
		NetworkMode:    "none",
		ReadonlyRootfs: true,
		CapDrop:        []string{"ALL"},
		SecurityOpt:    []string{"no-new-privileges"},
		Tmpfs: map[string]string{
			"/box": tmpfsOptions(req.Spec.TmpfsSizeBytes, req.Spec.TmpfsExec),
		},
		Resources: container.Resources{
			Memory:     req.Compile.MemoryBytes,
			MemorySwap: req.Compile.MemoryBytes, // equal to Memory = swap disabled
			NanoCPUs:   req.Compile.NanoCPUs,
			PidsLimit:  &pids,
			Ulimits: []*container.Ulimit{
				// No core dumps: a crashing submission must not fill the
				// tmpfs (or the host's core_pattern target).
				{Name: "core", Soft: 0, Hard: 0},
				{Name: "nofile", Soft: sandboxNofile, Hard: sandboxNofile},
			},
		},
	}

	resp, err := e.cli.ContainerCreate(ctx, cfg, hostCfg, nil, nil, "")
	if err != nil {
		return "", fmt.Errorf("sandbox: create container from %q (missing image? run 'task executor:images'): %w",
			req.Spec.Image, err)
	}
	return resp.ID, nil
}

// containerLifetime is how long a sandbox container may live: every phase
// timeout of the request plus a grace period. The idle `sleep` exits after
// it, stopping the container even if the executor never comes back to it.
func containerLifetime(req Request) time.Duration {
	return req.CompileTimeout + req.RunTimeout + 2*sourceWriteTimeout + lifetimeGrace
}

// ReapStale force-removes sandbox containers created more than maxAge ago.
// Normal executions remove their own container on every path; this catches
// the ones an executor crash, OOM-kill or daemon hiccup left behind, so they
// cannot accumulate into a resource-exhaustion DoS. maxAge must exceed the
// longest legitimate execution so a live sandbox (possibly owned by another
// executor sharing the daemon) is never touched.
func (e *Engine) ReapStale(ctx context.Context, maxAge time.Duration) (int, error) {
	list, err := e.cli.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: filters.NewArgs(filters.Arg("label", sandboxLabel)),
	})
	if err != nil {
		return 0, fmt.Errorf("sandbox: list sandbox containers: %w", err)
	}

	cutoff := time.Now().Add(-maxAge)
	removed := 0
	for _, c := range list {
		if time.Unix(c.Created, 0).After(cutoff) {
			continue
		}
		e.removeContainer(c.ID)
		removed++
	}
	return removed, nil
}

// RunJanitor calls ReapStale immediately and then every interval until ctx
// is cancelled. Failures are logged, never fatal.
func (e *Engine) RunJanitor(ctx context.Context, interval, maxAge time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		n, err := e.ReapStale(ctx, maxAge)
		switch {
		case err != nil && ctx.Err() == nil:
			e.log.Warn("reap stale sandbox containers", "error", err)
		case n > 0:
			e.log.Warn("removed stale sandbox containers", "count", n)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (e *Engine) updateResources(ctx context.Context, id string, r Resources) error {
	pids := r.PidsLimit
	_, err := e.cli.ContainerUpdate(ctx, id, container.UpdateConfig{
		Resources: container.Resources{
			Memory:     r.MemoryBytes,
			MemorySwap: r.MemoryBytes,
			NanoCPUs:   r.NanoCPUs,
			PidsLimit:  &pids,
		},
	})
	if err != nil {
		return fmt.Errorf("sandbox: update container resources: %w", err)
	}
	return nil
}

// oomKilled reports whether the kernel OOM killer fired inside the
// container's cgroup. Best-effort: an inspect failure is logged and treated
// as false — judge.Verdict has an exit-code fallback for that case.
func (e *Engine) oomKilled(id string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	inspect, err := e.cli.ContainerInspect(ctx, id)
	if err != nil {
		e.log.Warn("inspect container for oom status", "container", shortID(id), "error", err)
		return false
	}
	return inspect.State != nil && inspect.State.OOMKilled
}

// killContainer hard-kills the container; used to enforce timeouts. An
// "already stopped" style error is expected when the process exited between
// the deadline firing and the kill landing.
func (e *Engine) killContainer(id string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := e.cli.ContainerKill(ctx, id, "KILL"); err != nil {
		e.log.Debug("kill container", "container", shortID(id), "error", err)
	}
}

// removeContainer force-removes the container with a fresh context so
// cleanup happens even when the request context is already cancelled.
func (e *Engine) removeContainer(id string) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := e.cli.ContainerRemove(ctx, id, container.RemoveOptions{Force: true}); err != nil {
		// A leaked sandbox is a real problem: loud log level.
		e.log.Error("remove sandbox container", "container", shortID(id), "error", err)
	}
}

// tmpfsOptions renders the mount options for the /box tmpfs. Docker's
// defaults (noexec,nosuid,nodev) apply unless overridden; exec is granted
// only to languages that must run a compiled binary out of /box.
func tmpfsOptions(sizeBytes int64, exec bool) string {
	execOpt := "noexec"
	if exec {
		execOpt = "exec"
	}
	return fmt.Sprintf("rw,%s,nosuid,nodev,size=%d,mode=1777", execOpt, sizeBytes)
}

func shortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}
