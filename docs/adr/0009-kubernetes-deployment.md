# ADR-0009: Kubernetes deployment — DinD judging sidecar, umbrella chart, local-first Terraform

- Status: accepted
- Date: 2026-06-11
- Phase: 5
- Builds on: [ADR-0005](0005-docker-sandbox-strategy.md) (sandbox model)

## Context

The platform must run on Kubernetes. Three sub-problems with real teeth:

1. The executor requires a Docker daemon, but Kubernetes nodes run
   containerd — there is no Docker socket, and mounting any node runtime
   socket hands pods node-root (explicitly forbidden by ADR-0005).
2. PostgreSQL/Redis need to exist in-cluster for dev without adopting a
   chart dependency that can rot.
3. "Terraform" must mean something verifiable on a machine with no cloud
   account.

## Decisions

### 1. The executor pod hosts its own Docker daemon (DinD native sidecar)

```
executor pod
├── init: copy-sandbox-images   (Dockerfiles out of the executor image)
├── init: dind                  (docker:dind, restartPolicy: Always —
│                                native sidecar; the ONLY privileged
│                                container in the system)
├── init: build-sandbox-images  (one-shot docker build vs the sidecar;
│                                DinD storage on a PVC → builds once
│                                per cluster, not per restart)
└── executor                    (unprivileged; DOCKER_HOST=tcp://127.0.0.1:2375)
```

- **Zero code changes**: the daemon endpoint was configuration from day one
  (ADR-0005's deliberate bet). The same executor binary runs on a Windows
  named pipe, a Linux socket, and now a pod-local TCP daemon.
- **Privilege containment**: `privileged: true` exists only on the dind
  container, inside the one pod whose job is hosting hostile code — the
  "executor hosts are a separate security zone" boundary, realized.
  Untrusted code still runs under every Phase-2 sandbox invariant, now two
  container layers below the node.
- **Rejected**: CRI/containerd rewrite (different API, node-root socket);
  Job-per-submission (pod startup latency per test case kills judge
  latency); gVisor/Kata runtime classes (unavailable on Kind/dev machines —
  still the right production hardening step later).
- TLS between executor and dind is off: they share a network namespace;
  there is no wire to protect.

### 2. Hand-rolled minimal PostgreSQL/Redis manifests

Bitnami's free image catalog was gutted in 2025; a chart dependency that can
rot under its maintainer is worse than ~80 lines of StatefulSet we own. The
chart's `postgres.externalDatabaseUrl` value is the managed-database seam —
set it and the StatefulSet isn't rendered. Redis stays a Deployment with no
persistence, encoding ADR-0004 in infrastructure a second time.

### 3. One umbrella chart; Terraform targets the environment that exists

Two services and two data stores don't justify four charts; one chart with
per-component values (images, resources, toggles) stays reviewable.

Terraform provisions the **local Kind environment** end to end (tehcyx/kind
provider → cluster; helm provider → release; `kind load` via local-exec for
image side-loading, the one place IaC must shell out because Kind has no
registry). This is real, applied, destroyed, verifiable IaC. A cloud root
module is deliberately absent: with no cloud account, AWS HCL would be
untested dead code — the seams it would use (external DB URL, registry
images, secret overrides) are in place and documented instead.

## Consequences

- The executor is a **singleton per cluster** for now (RWO image-cache PVC,
  dev-machine capacity); judge throughput scaling is Phase 7's problem and
  will likely arrive together with the durable queue.
- First executor start per cluster pays ~3–6 min of in-DinD sandbox image
  builds; afterwards the PVC makes restarts fast. Production path: push
  sandbox images to a registry, replace the build init with pulls.
- The seed Job is a Helm post-install/post-upgrade hook gated by
  `seed.enabled` — convenient for dev/CI, must be disabled for real
  deployments.
- CI gains the strongest claim the repo can make: every push deploys the
  chart into a fresh Kind cluster and a submission is judged `accepted`
  inside it.
