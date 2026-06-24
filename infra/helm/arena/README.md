# arena Helm chart

Deploys the full platform: api-gateway, executor (with its Docker-in-Docker
judging sidecar — see [ADR-0009](../../../docs/adr/0009-kubernetes-deployment.md)),
PostgreSQL, and Redis.

Local usage is wrapped by tasks (`task k8s:up && task k8s:deploy`); see the
Quickstart in the [root README](../../../README.md).

## Production notes (what you'd change beyond a dev cluster)

- **Database**: set `postgres.externalDatabaseUrl` to a managed instance
  (RDS/Cloud SQL); the in-cluster StatefulSet then isn't rendered.
- **Secrets**: `auth.jwtSecret` / `postgres.password` ship dev-grade
  defaults. Override via `--set`, or own the `<release>-secrets` Secret with
  external-secrets/SOPS — the chart only needs the name and keys to match.
- **Sandbox images**: the executor pod builds them in DinD on first start
  (fine for dev clusters). At scale, push `arena-sandbox-*` to a registry
  and replace the build init container with pulls.
- **Executor scaling**: a singleton by design until Phase 7 (RWO image-cache
  PVC, dev-machine capacity). The gateway scales horizontally already
  (Redis Pub/Sub fan-out).
- **Ingress/TLS**: the chart exposes a NodePort for Kind; put a real Ingress
  + cert-manager in front for anything public, and set
  `config.frontendOrigin` accordingly.
