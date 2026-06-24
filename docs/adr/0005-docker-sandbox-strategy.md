# ADR-0005: Sandbox untrusted code in ephemeral sibling Docker containers

- Status: accepted (implementation lands in Phase 2)
- Date: 2026-06-11
- Phase: 1 (decision), 2 (implementation)

## Context

Arena executes untrusted user code in C++, Python, and Go. This is the
platform's defining security problem: submissions must be assumed malicious
(fork bombs, infinite loops, network exfiltration attempts, filesystem
attacks, privilege escalation).

Options considered:

1. **OS-level sandboxing on the host** (seccomp + rlimits + namespaces by
   hand, or nsjail/isolate): strongest control, but a large amount of
   security-critical code to get right, and poor portability to a
   Windows-hosted dev environment.
2. **Ephemeral Docker containers** (one per execution, sibling containers via
   the Docker API): containers are not a perfect security boundary, but with
   network disabled, read-only rootfs, non-root user, dropped capabilities,
   and strict cgroup quotas they raise the cost of escape far beyond this
   threat model, while staying debuggable and portable.
3. **MicroVMs (Firecracker / gVisor)**: the strongest isolation, but adds
   heavy operational complexity and does not run on the local Windows + Docker
   Desktop dev environment at all.

## Decision

Option 2. The executor talks to the Docker daemon and, per execution, creates
a short-lived container from a prebuilt per-language image with:

- `NetworkMode: none` (no network)
- read-only root filesystem; code and stdin delivered without host bind mounts
- non-root user, all capabilities dropped, `no-new-privileges`
- memory limit (128 MB), CPU quota (0.5 CPU), pids limit, wall-clock timeout
  (2 s default) enforced by the executor
- container force-removed after every run

The executor creates **sibling** containers (it talks to the host's Docker
daemon) rather than Docker-in-Docker, avoiding privileged mode.

## Rationale

- Matches the threat model: contestants attacking a judge, not nation-state
  VM-escape research.
- Runs identically-enough across local dev (Docker Desktop named pipe on
  Windows) and Kubernetes (daemon socket on Linux nodes) because the daemon
  endpoint is configuration, not code.
- Per-language prebuilt images keep per-execution latency to container
  start-up cost rather than image build cost.

## Consequences

- The Docker daemon becomes a hard runtime dependency of the executor, and
  access to its socket is root-equivalent — the socket must never be exposed
  to the sandboxed containers themselves, and executor hosts/nodes are
  treated as a separate security zone.
- Container start-up latency (~hundreds of ms) bounds judge latency; queueing
  (Phase 7) absorbs bursts.
- A future hardening step (gVisor runtime class in Kubernetes) slots in
  without changing the executor's code, only its deployment.
