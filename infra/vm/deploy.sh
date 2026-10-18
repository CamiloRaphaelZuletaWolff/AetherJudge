#!/usr/bin/env bash
# (Re)deploy the executor stack on the VM: pull the images published by
# .github/workflows/executor-images.yml, retag the sandbox images to the
# names the executor resolves, and (re)start executor + Caddy.
#
#   bash ~/aether/infra/vm/deploy.sh            # tag from .env (IMAGE_TAG, default latest)
#   bash ~/aether/infra/vm/deploy.sh 3f2a1bc    # a specific build (short commit SHA)
#
# Idempotent; .github/workflows/deploy-executor.yml runs exactly this over SSH
# after a `git pull`. Configuration comes from infra/vm/.env (see .env.example).
set -euo pipefail

cd "$(dirname "$(readlink -f "$0")")"

log() { printf '\n\033[1;34m==> %s\033[0m\n' "$*"; }
die() {
	printf '\033[1;31mERROR: %s\033[0m\n' "$*" >&2
	exit 1
}

[ -f .env ] || die "infra/vm/.env not found. Run: cp .env.example .env && nano .env"
set -a
# shellcheck disable=SC1091
. ./.env
set +a

GHCR_OWNER="$(printf '%s' "${GHCR_OWNER:-camiloraphaelzuletawolff}" | tr '[:upper:]' '[:lower:]')"
# Optional first argument overrides IMAGE_TAG from .env (the deploy workflow
# passes the short commit SHA it just built):  deploy.sh 3f2a1bc
IMAGE_TAG="${1:-${IMAGE_TAG:-latest}}"
export GHCR_OWNER IMAGE_TAG

[ -n "${EXECUTOR_AUTH_TOKEN:-}" ] || die "EXECUTOR_AUTH_TOKEN is empty in .env (generate one: openssl rand -base64 32)"
[ "${#EXECUTOR_AUTH_TOKEN}" -ge 32 ] || die "EXECUTOR_AUTH_TOKEN must be at least 32 characters"
case "${EXECUTOR_DOMAIN:-}" in
"" | 203-0-113-7.sslip.io) die "set EXECUTOR_DOMAIN in .env to <your-public-ip-with-dashes>.sslip.io" ;;
esac

docker info >/dev/null 2>&1 || die "cannot talk to Docker. Did you log out/in after bootstrap.sh (docker group)?"

# The executor container (uid 10001) joins the host docker group by GID to
# reach /var/run/docker.sock.
DOCKER_GID="$(getent group docker | cut -d: -f3)"
[ -n "${DOCKER_GID}" ] || die "no 'docker' group on this host - run bootstrap.sh first"
export DOCKER_GID

if [ -n "${GHCR_TOKEN:-}" ]; then
	log "Logging in to ghcr.io as ${GHCR_USER:?GHCR_USER must be set with GHCR_TOKEN} (private packages)"
	printf '%s' "${GHCR_TOKEN}" | docker login ghcr.io -u "${GHCR_USER}" --password-stdin
fi

registry="ghcr.io/${GHCR_OWNER}"
log "Pulling images from ${registry} (tag ${IMAGE_TAG}, arch $(uname -m))"
docker pull "${registry}/aether-executor:${IMAGE_TAG}"
for lang in cpp python go; do
	docker pull "${registry}/aether-sandbox-${lang}:${IMAGE_TAG}"
	# The executor looks sandboxes up locally as arena-sandbox-<lang>:<EXECUTOR_IMAGE_TAG>
	# (backend/services/executor/internal/lang) and never pulls them itself.
	docker tag "${registry}/aether-sandbox-${lang}:${IMAGE_TAG}" "arena-sandbox-${lang}:${IMAGE_TAG}"
done

log "Starting executor + caddy"
docker compose up -d --remove-orphans

log "Pruning dangling images"
docker image prune -f >/dev/null

log "Status"
docker compose ps

# Caddy answers anything outside the two gRPC services with 404, so a 404
# over verified HTTPS proves DNS (sslip.io), the open ports and the Let's
# Encrypt certificate in one request. The first issuance can take ~30 s.
log "Checking https://${EXECUTOR_DOMAIN}/ (expect HTTP 404 with a valid certificate)"
code=000
for _ in $(seq 1 12); do
	code="$(curl -sS -o /dev/null -w '%{http_code}' --max-time 5 "https://${EXECUTOR_DOMAIN}/" 2>/dev/null || true)"
	[ "${code}" = "404" ] && break
	sleep 5
done
if [ "${code}" = "404" ]; then
	log "OK: TLS is live. On Render set EXECUTOR_ADDR=${EXECUTOR_DOMAIN}:443 and EXECUTOR_TLS=true"
else
	# Some clouds do not hairpin a VM's traffic to its own public IP, so a
	# failure here is not conclusive: retry from another machine.
	printf '\033[1;33mWARN: no valid HTTPS answer yet (last status %s). Retry from your PC: curl -v https://%s/ - if that fails too, check ports 80/443 (cloud firewall + iptables) and: docker compose logs caddy\033[0m\n' "${code}" "${EXECUTOR_DOMAIN}" >&2
fi

docker compose logs --tail 20 executor
