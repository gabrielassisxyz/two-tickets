#!/usr/bin/env bash
# Playwright link crawl of the production `web` image against a real api.
# It builds both images the way deploy/web-e2e.sh does, signs in through
# the login form on a mobile viewport with touch input, then follows every
# internal link and proves none answers 404 or an error page. The library
# specs (web/tests/library) run in the same command, against the same stack.
#
# The api's TMDB is a substitute (deploy/e2e/tmdb-substitute.mjs) answering
# from the recorded fixtures in testdata/tmdb/, so cards and title pages
# render without a TMDB key or a network call to TMDB.
#
# The crawl runs inside the official Playwright image
# (mcr.microsoft.com/playwright), on the same Docker network, so no browser
# is installed on the host, locally or in CI. The @playwright/test version
# in web/package.json is pinned to the same version as the image tag.
#
# Everything it starts is removed in a trap, whatever happens.
set -euo pipefail
cd "$(dirname "$0")/.."

playwright_version="$(node -p "require('./web/package.json').devDependencies['@playwright/test']")"
image="two-tickets-web-crawl:$$"
network="two-tickets-crawl-${$}"
work="$(mktemp -d)"
containers=()

cleanup() {
  local c
  for c in "${containers[@]}"; do
    docker rm -f "$c" >/dev/null 2>&1 || true
  done
  docker network rm "$network" >/dev/null 2>&1 || true
  docker rmi -f "$image" >/dev/null 2>&1 || true
  rm -rf "$work"
}
trap cleanup EXIT

fail() {
  echo "web crawl: $*" >&2
  exit 1
}

password="sentinelsentinelsentinel"

echo "==> web crawl: building the production image and the api binary"
docker build -q -f deploy/web.Dockerfile -t "$image" web/ >/dev/null
CGO_ENABLED=0 go build -o "$work/two-tickets" ./cmd/two-tickets
password_hash="$(printf '%s' "$password" | go run ./deploy/e2e/hashpassword)"

docker network create "$network" >/dev/null
subnet="$(docker network inspect -f '{{(index .IPAM.Config 0).Subnet}}' "$network")"

tmdb="two-tickets-crawl-tmdb-${$}"
containers+=("$tmdb")
docker run -d --name "$tmdb" --network "$network" --network-alias tmdb \
  -v "$PWD/deploy/e2e/tmdb-substitute.mjs:/e2e/tmdb-substitute.mjs:ro" \
  -v "$PWD/testdata/tmdb:/e2e/tmdb:ro" \
  --entrypoint node "$image" /e2e/tmdb-substitute.mjs >/dev/null

api="two-tickets-crawl-api-${$}"
containers+=("$api")
docker run -d --name "$api" --network "$network" --network-alias api \
  --tmpfs /data:mode=1777 \
  -v "$work/two-tickets:/e2e/two-tickets:ro" \
  -e DATA_DIR=/data -e LISTEN_ADDR=:8080 \
  -e TMDB_API_KEY=sentinelsentinelsentinel \
  -e TMDB_BASE_URL=http://tmdb:8080/3 \
  -e ACCOUNT_1_KEY=first -e ACCOUNT_1_USERNAME=alice \
  -e ACCOUNT_1_EMAIL=alice@example.com -e ACCOUNT_1_DISPLAY_NAME=Alice \
  -e "ACCOUNT_1_PASSWORD_HASH=${password_hash}" \
  -e ACCOUNT_2_KEY=second -e ACCOUNT_2_USERNAME=bob \
  -e ACCOUNT_2_EMAIL=bob@example.com -e ACCOUNT_2_DISPLAY_NAME=Bob \
  -e "ACCOUNT_2_PASSWORD_HASH=${password_hash}" \
  -e ALLOWED_ORIGINS=http://web:3000 \
  -e "TRUSTED_PROXY_CIDRS=${subnet}" \
  --entrypoint /e2e/two-tickets "$image" >/dev/null

web="two-tickets-crawl-web-${$}"
containers+=("$web")
docker run -d --name "$web" --network "$network" --network-alias web \
  --read-only --tmpfs /tmp \
  -e PORT=3000 -e HOSTNAME=0.0.0.0 -e "API_INTERNAL_URL=http://api:8080" \
  -p '127.0.0.1::3000' "$image" >/dev/null
port="$(docker port "$web" 3000 | head -n 1 | sed 's/.*://')"

echo "==> web crawl: waiting for the server on 127.0.0.1:${port}"
ready=0
for _ in $(seq 1 60); do
  if curl -sSf -o /dev/null "http://127.0.0.1:${port}/favicon.ico"; then
    ready=1
    break
  fi
  sleep 1
done
if [ "$ready" -ne 1 ]; then
  docker logs "$web" >&2 || true
  fail "web did not become ready"
fi

# The specs import @playwright/test, which resolves from the spec's own
# directory upwards. web/node_modules exists only where someone installed it,
# never on the CI runner, so the specs are copied into a scratch project that
# installs the pinned package itself, and the run never depends on the host.
# The script expands inside the container, where PLAYWRIGHT_VERSION is set.
# shellcheck disable=SC2016
crawl_in_container='set -e
mkdir -p /tmp/crawl/tests
cp -r tests/crawl tests/library tests/recs /tmp/crawl/tests/
cd /tmp/crawl
npm init -y >/dev/null
npm install --no-audit --no-fund --loglevel=error "@playwright/test@${PLAYWRIGHT_VERSION}"
npx playwright test tests/crawl tests/library tests/recs --workers=1 --output=/tmp/test-results'

echo "==> web crawl: running the link crawl and the library and recommendations specs (image v${playwright_version})"
docker run --rm --network "$network" \
  -v "$PWD/web:/work:ro" \
  -w /work \
  -e PLAYWRIGHT_VERSION="$playwright_version" \
  -e CRAWL_BASE_URL=http://web:3000 \
  -e CRAWL_USERNAME=alice \
  -e "CRAWL_PASSWORD=${password}" \
  "mcr.microsoft.com/playwright:v${playwright_version}-noble" \
  sh -c "$crawl_in_container" \
  || fail "the Playwright suite failed: a broken internal link, a library spec, or the suite did not run (see above)"

echo "web crawl: green"
