# ADR-0006: Exec-driven execution pipeline with per-phase resource limits

- Status: accepted
- Date: 2026-06-11
- Phase: 2
- Builds on: [ADR-0005](0005-docker-sandbox-strategy.md)

## Context

ADR-0005 chose ephemeral sibling Docker containers as the sandbox. This ADR
fixes the concrete pipeline: how code gets in, how output gets out, and how
compile and run phases are bounded. Three constraints interact:

1. **No host filesystem contact and a read-only rootfs** rule out both bind
   mounts and `docker cp` (which conflicts with `ReadonlyRootfs`).
2. **Compile and run need different budgets.** `g++ -O2` with
   `<bits/stdc++.h>` exceeds 128 MB on its own; naive uniform limits would
   reject every realistic C++ submission.
3. **tmpfs pages are charged to the memory cgroup**, so anything large left
   in `/box` after compile (Go's build cache) eats the run-phase limit.

## Decision

One container per submission, driven entirely through `docker exec`:

1. **Create** from a prebuilt per-language image with `sleep` as the
   entrypoint, network `none`, read-only rootfs, tmpfs `/box`
   (`noexec` unless the language must run a binary from it), non-root user,
   all capabilities dropped, `no-new-privileges`, pids limit, and
   *compile-phase* memory/CPU.
2. **Write source** via an exec with stdin attached (`cat > /box/<src>`) —
   code, stdin, and output all travel over the Docker API socket; nothing
   touches the host filesystem.
3. **Compile** as its own exec under the compile budget. Python's "compile"
   is `py_compile`, so syntax errors judge as `COMPILATION_ERROR` like every
   other language.
4. **Cleanup + shrink**: delete build caches from the tmpfs, then
   `ContainerUpdate` the *same* container down to strict run limits
   (128 MB / 0.5 CPU / 64 pids by default).
5. **Run** as a final exec with the test stdin; wall-clock timeout enforced
   by killing the container; stdout/stderr captured through hard byte caps
   that keep draining (an output flood cannot balloon executor memory).
6. **Remove** (force) in a defer on every path.

MLE detection: container `OOMKilled` flag, with exit 137 (SIGKILL) absent a
timeout as fallback — cgroup OOM reporting differs between WSL2 and CI
kernels, and both paths are integration-tested.

Image-level performance work that made the budgets real (measured on the dev
machine, 2 CPUs / 512 MB compile budget):

| Optimization | Before | After |
| --- | --- | --- |
| Go: pre-warmed stdlib build cache baked into image, copied to tmpfs per run | 73 s | **0.7 s** |
| C++: precompiled `<bits/stdc++.h>` header (`.gch`) in image | 21.3 s | **2.3 s** |

## Alternatives rejected

- **Two containers per submission** (fat compile box → transfer binary →
  strict run box): also correct, but adds a second container lifecycle and a
  binary hand-off per submission for no security gain over
  `ContainerUpdate`.
- **`docker cp` for code delivery**: blocked by read-only rootfs; would also
  require relaxing it.
- **CPU-time accounting for timeouts**: MVP measures wall-clock around the
  run exec. Fair enough for now; revisit with Phase 7 load work.

## Consequences

- The executor reports `memory_used_kb = 0` for now: cgroup-v2 peak metrics
  are not reliably readable across WSL2/CI from outside the container, and
  the MLE *verdict* never depends on measurement (OOM kill flag). The proto
  field is documented as best-effort.
- `sleep` as PID 1 does not reap zombies; irrelevant at this lifetime (the
  container lives seconds and is force-removed), and it incidentally makes
  fork bombs die faster (zombies pin the pids quota).
- Every sandbox container carries an `arena.sandbox=1` label; integration
  tests assert zero labeled containers remain after each test, turning
  "auto-destroy" from a promise into a checked invariant.
