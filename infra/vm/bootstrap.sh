#!/usr/bin/env bash
# One-time preparation of a fresh executor VM (Ubuntu 22.04/24.04 or Debian
# 12, amd64 or arm64). Safe to re-run: every step checks before it changes.
#
#   curl -fsSL https://raw.githubusercontent.com/CamiloRaphaelZuletaWolff/AetherJudge/main/infra/vm/bootstrap.sh | bash
#   # or, from a clone:  bash infra/vm/bootstrap.sh
#
# What it does:
#   1. Adds a 4 GB swap file when the machine has < 2 GB of RAM (e2-micro).
#   2. Opens TCP 80/443 in the host firewall on Oracle Cloud's Ubuntu images,
#      which ship a restrictive iptables policy (see the comment there).
#   3. Installs Docker CE + the compose plugin from Docker's official repo.
#   4. Adds the current user to the docker group.
#   5. Clones (or updates) the repository into ~/aether.
#
# Run it as your normal SSH user (ubuntu / your Google account name); it uses
# sudo where needed.
set -euo pipefail

REPO_URL="${REPO_URL:-https://github.com/CamiloRaphaelZuletaWolff/AetherJudge.git}"
REPO_DIR="${REPO_DIR:-$HOME/aether}"
REPO_BRANCH="${REPO_BRANCH:-main}"
SWAP_SIZE_GB="${SWAP_SIZE_GB:-4}"

log() { printf '\n\033[1;34m==> %s\033[0m\n' "$*"; }
warn() { printf '\033[1;33mWARN: %s\033[0m\n' "$*" >&2; }

if [ "$(id -u)" -eq 0 ]; then
	warn "running as root: the docker group step will target root. Prefer your normal SSH user."
fi

# shellcheck disable=SC1091
. /etc/os-release
case "${ID}" in
ubuntu | debian) ;;
*)
	echo "Unsupported distribution '${ID}'. Use Ubuntu 22.04/24.04 or Debian 12." >&2
	exit 1
	;;
esac
log "Detected ${PRETTY_NAME} on $(dpkg --print-architecture)"

# --- 1. swap ------------------------------------------------------------------
# The compile phase may use up to 512 MB inside the sandbox on top of the
# executor, dockerd and Caddy. On 1 GB that only survives with swap.
mem_kb="$(awk '/MemTotal/ {print $2}' /proc/meminfo)"
if [ "${mem_kb}" -lt 2000000 ]; then
	if swapon --show=NAME --noheadings | grep -q '^/swapfile$'; then
		log "Swap: /swapfile already active"
	else
		log "Swap: RAM is $((mem_kb / 1024)) MB (< 2 GB) - creating a ${SWAP_SIZE_GB} GB /swapfile"
		sudo fallocate -l "${SWAP_SIZE_GB}G" /swapfile || sudo dd if=/dev/zero of=/swapfile bs=1M count=$((SWAP_SIZE_GB * 1024))
		sudo chmod 600 /swapfile
		sudo mkswap /swapfile
		sudo swapon /swapfile
		grep -q '^/swapfile ' /etc/fstab || echo '/swapfile none swap sw 0 0' | sudo tee -a /etc/fstab >/dev/null
		echo 'vm.swappiness=10' | sudo tee /etc/sysctl.d/99-aether-swap.conf >/dev/null
		sudo sysctl -q -p /etc/sysctl.d/99-aether-swap.conf
	fi
else
	log "Swap: RAM is $((mem_kb / 1024)) MB - no swap needed"
fi

# --- 2. host firewall (Oracle Cloud Ubuntu images) -----------------------------
# Oracle's platform images ship /etc/iptables/rules.v4 whose INPUT chain
# accepts only SSH and ends in "-j REJECT --reject-with icmp-host-prohibited".
# Opening 80/443 in the VCN Security List is therefore NOT enough on its own:
# packets reach the VM and are rejected by the VM itself. Docker-published
# ports are DNAT-ed through the FORWARD chain (where Docker installs its own
# rules), but we also accept 80/443 on INPUT so ACME and HTTPS work no matter
# how Caddy is run. We edit rules.v4 directly instead of using
# `netfilter-persistent save`, which would also persist Docker's runtime
# chains and cause duplicate/stale rules after a reboot.
RULES=/etc/iptables/rules.v4
if [ -f "${RULES}" ] && sudo grep -q 'icmp-host-prohibited' "${RULES}"; then
	log "Firewall: Oracle-style iptables policy found - opening TCP 80 and 443"
	for port in 443 80; do
		if ! sudo grep -q -- "--dport ${port} -j ACCEPT" "${RULES}"; then
			sudo sed -i "/--dport 22 -j ACCEPT/a -A INPUT -p tcp -m state --state NEW -m tcp --dport ${port} -j ACCEPT" "${RULES}"
		fi
		if ! sudo iptables -C INPUT -p tcp -m state --state NEW -m tcp --dport "${port}" -j ACCEPT 2>/dev/null; then
			sudo iptables -I INPUT 1 -p tcp -m state --state NEW -m tcp --dport "${port}" -j ACCEPT
		fi
	done
else
	log "Firewall: no Oracle-style REJECT policy - nothing to open on the host (use the cloud firewall)"
fi

# --- 3. Docker CE ---------------------------------------------------------------
if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
	log "Docker: already installed ($(docker --version))"
else
	log "Docker: installing Docker CE from download.docker.com"
	sudo apt-get update -y
	sudo apt-get install -y ca-certificates curl git
	sudo install -m 0755 -d /etc/apt/keyrings
	sudo curl -fsSL "https://download.docker.com/linux/${ID}/gpg" -o /etc/apt/keyrings/docker.asc
	sudo chmod a+r /etc/apt/keyrings/docker.asc
	echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/${ID} ${VERSION_CODENAME} stable" |
		sudo tee /etc/apt/sources.list.d/docker.list >/dev/null
	sudo apt-get update -y
	sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
fi
sudo systemctl enable --now docker >/dev/null
command -v git >/dev/null 2>&1 || sudo apt-get install -y git

# --- 4. docker group ------------------------------------------------------------
if id -nG "${USER}" | tr ' ' '\n' | grep -qx docker; then
	log "Docker group: ${USER} is already a member"
else
	log "Docker group: adding ${USER} (log out and back in for it to apply)"
	sudo usermod -aG docker "${USER}"
	NEED_RELOGIN=1
fi

# --- 5. repository --------------------------------------------------------------
if [ -d "${REPO_DIR}/.git" ]; then
	log "Repo: updating ${REPO_DIR}"
	git -C "${REPO_DIR}" fetch --depth 1 origin "${REPO_BRANCH}"
	git -C "${REPO_DIR}" checkout -q -B "${REPO_BRANCH}" FETCH_HEAD
else
	log "Repo: cloning ${REPO_URL} (${REPO_BRANCH}) into ${REPO_DIR}"
	git clone --depth 1 --branch "${REPO_BRANCH}" "${REPO_URL}" "${REPO_DIR}"
fi
if [ ! -f "${REPO_DIR}/infra/vm/.env" ]; then
	cp "${REPO_DIR}/infra/vm/.env.example" "${REPO_DIR}/infra/vm/.env"
	chmod 600 "${REPO_DIR}/infra/vm/.env"
fi

ip4="$(curl -fsS4 --max-time 5 https://ifconfig.me 2>/dev/null || true)"
log "Done."
cat <<EOF

Next steps:
  1. ${NEED_RELOGIN:+Log out and SSH back in (docker group). Then: }nano ${REPO_DIR}/infra/vm/.env
       EXECUTOR_DOMAIN=${ip4:+${ip4//./-}.sslip.io}${ip4:-<ip-with-dashes>.sslip.io}
       EXECUTOR_AUTH_TOKEN=<openssl rand -base64 32>
  2. bash ${REPO_DIR}/infra/vm/deploy.sh
EOF
