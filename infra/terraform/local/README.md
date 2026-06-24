# Terraform — local environment

Provisions the complete Arena stack from nothing on a developer machine:
a Kind cluster (with the gateway port mapping) plus the `arena` Helm release.

```bash
task tf:init      # once
task tf:apply     # builds images, creates cluster, installs the release
task k8s:smoke    # judged submission against http://localhost:8091
task tf:destroy
```

State is local (`terraform.tfstate`, gitignored) — appropriate for a
single-developer environment; a team would use a remote backend.

## Where the cloud seam goes

This module proves the IaC workflow against infrastructure that actually
exists. A cloud root module (e.g. `infra/terraform/aws/`) would reuse the
same Helm chart and replace the pieces below — it is deliberately NOT
checked in here, because untested Terraform is worse than absent Terraform:

| Local | Cloud replacement |
| --- | --- |
| `kind_cluster` | EKS/GKE module |
| `kind load docker-image` | ECR/GAR + CI image pushes |
| In-cluster PostgreSQL StatefulSet | RDS/Cloud SQL via `postgres.externalDatabaseUrl` |
| Helm-templated dev Secret | Secrets Manager + external-secrets |
| NodePort + host mapping | Ingress + LB + cert-manager |
