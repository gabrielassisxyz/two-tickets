#!/usr/bin/env bash
# Smoke test for the production `web` image. It builds the image from
# `deploy/web.Dockerfile`, starts it with a read-only root filesystem and the
# same mounts production uses, then fetches one JS chunk, one CSS file, the
# logo and the favicon. The container and the image are removed in a trap,
# whatever happens.
set -euo pipefail
cd "$(dirname "$0")/.."

image="two-tickets-web-smoke:$$"
container=""

cleanup() {
  if [ -n "${container:-}" ]; then
    docker rm -f "$container" >/dev/null 2>&1 || true
  fi
  docker rmi -f "$image" >/dev/null 2>&1 || true
}
trap cleanup EXIT

echo "==> web smoke: building production image"
docker build -f deploy/web.Dockerfile -t "$image" web/

echo "==> web smoke: starting with a read-only root filesystem"
container="$(docker run -d --read-only --tmpfs /tmp \
  -e PORT=3000 -e HOSTNAME=0.0.0.0 \
  -p '127.0.0.1::3000' "$image")"

host_port="$(docker port "$container" 3000 | sed 's/.*://')"
base="http://127.0.0.1:${host_port}"

echo "==> web smoke: waiting for the server on ${base}"
ready=0
for _ in $(seq 1 60); do
  if curl -sSf -o /dev/null "${base}/favicon.ico"; then
    ready=1
    break
  fi
  sleep 1
done
if [ "$ready" -ne 1 ]; then
  echo "web smoke: server did not become ready" >&2
  docker logs "$container" >&2 || true
  exit 1
fi

# Rendered pages call the `api` process, which does not run here, so `/` can
# answer 500 while static assets stay servable. Discover the chunk and CSS
# paths from the files baked into the image instead of from rendered HTML.
echo "==> web smoke: discovering one JS chunk and one CSS file"
js_file="$(docker run --rm "$image" find .next/standalone/.next/static/chunks -name '*.js' 2>/dev/null | head -n 1 || true)"
css_file="$(docker run --rm "$image" find .next/standalone/.next/static -name '*.css' 2>/dev/null | head -n 1 || true)"
if [ -z "$js_file" ]; then
  echo "web smoke: no JS chunk baked into the image" >&2
  exit 1
fi
if [ -z "$css_file" ]; then
  echo "web smoke: no CSS file baked into the image" >&2
  exit 1
fi
js_path="/_next/static/${js_file#.next/standalone/.next/static/}"
css_path="/_next/static/${css_file#.next/standalone/.next/static/}"
echo "web smoke: chunk ${js_path}, css ${css_path}"

check_200() {
  local path="$1"
  local code
  code="$(curl -sS -o /dev/null -w '%{http_code}' "${base}${path}")"
  if [ "$code" != "200" ]; then
    echo "web smoke: expected HTTP 200 for ${path}, got ${code}" >&2
    exit 1
  fi
  echo "web smoke: 200 ${path}"
}

check_200 "$js_path"
check_200 "$css_path"
check_200 "/logo_full.png"
check_200 "/favicon.ico"

echo "web smoke: green"
