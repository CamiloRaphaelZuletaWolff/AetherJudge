# Security Policy

Arena executes untrusted, attacker-supplied code as its core function, so
security is treated as a first-class concern. The full analysis lives in
[docs/security/threat-model.md](docs/security/threat-model.md).

## Reporting a vulnerability

Please report security issues **privately** — do not open a public issue for
anything exploitable.

- Use GitHub's **"Report a vulnerability"** flow (Security → Advisories) on this
  repository, or
- email the maintainer listed on the GitHub profile.

Please include: a description, reproduction steps or a proof of concept, the
affected component, and the potential impact. You'll get an acknowledgement and,
where applicable, coordinated disclosure once a fix is available.

## Scope

In scope: the api-gateway, the executor and its sandbox isolation, the auth and
session design, and the deployment manifests in this repository.

Out of scope (see the threat model for rationale): edge WAF/DDoS protection,
issues requiring a host kernel 0-day against Docker's default seccomp profile,
and anything in third-party hosting platforms.

## Supported versions

This is a portfolio project; only the `main` branch is supported and security
fixes land there.
