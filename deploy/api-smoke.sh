#!/usr/bin/env bash
# Smoke test for the production `api` image. Starts the given image with an
# empty data volume and a throwaway account, then proves /healthz answers.
# The container is removed in a trap, whatever happens.
set -euo pipefail
cd "$(dirname "$0")/.."

image="${1:?usage: api-smoke.sh <image>}"
container=""

cleanup() {
  if [ -n "${container:-}" ]; then
    docker rm -f "$container" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

password="sentinelsentinelsentinel"
password_hash="$(printf '%s' "$password" | go run ./deploy/e2e/hashpassword)"

echo "==> api smoke: starting ${image}"
container="$(docker run -d --platform linux/amd64 --tmpfs /data \
  -e LISTEN_ADDR=:8080 -e DATA_DIR=/data -e TMDB_API_KEY=dummy \
  -e ACCOUNT_1_KEY=smokea -e ACCOUNT_1_USERNAME=smokea -e ACCOUNT_1_EMAIL=smokea@example.com \
  -e ACCOUNT_1_DISPLAY_NAME=smokea -e "ACCOUNT_1_PASSWORD_HASH=${password_hash}" \
  -e ACCOUNT_2_KEY=smokeb -e ACCOUNT_2_USERNAME=smokeb -e ACCOUNT_2_EMAIL=smokeb@example.com \
  -e ACCOUNT_2_DISPLAY_NAME=smokeb -e "ACCOUNT_2_PASSWORD_HASH=${password_hash}" \
  -e ALLOWED_ORIGINS=http://127.0.0.1 \
  -p '127.0.0.1::8080' "$image")"

port="$(docker port "$container" 8080 | head -n 1 | sed 's/.*://')"
base="http://127.0.0.1:${port}"

echo "==> api smoke: waiting for /healthz on ${base}"
ready=0
for _ in $(seq 1 60); do
  if curl -sSf -o /dev/null "${base}/healthz"; then
    ready=1
    break
  fi
  sleep 1
done
if [ "$ready" -ne 1 ]; then
  echo "api smoke: /healthz never answered" >&2
  docker logs "$container" >&2 || true
  exit 1
fi

echo "api smoke: green"
