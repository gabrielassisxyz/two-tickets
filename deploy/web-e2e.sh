#!/usr/bin/env bash
# End-to-end check of the production `web` image against an api, with the
# api's address given only at run time (API_INTERNAL_URL), never at build.
#
# Part one runs the image against a fake api that logs every request, and
# proves that server-side rendering calls API_INTERNAL_URL and nothing else,
# and that the /api/v1 proxy route keeps an encoded query and duplicate
# Set-Cookie lines intact inside the real Next server.
#
# Part two runs it against the real api binary, with a temporary SQLite
# directory and a throwaway account, and logs in the way the login page
# does: GET /login, POST /api/v1/auth/local through the proxy, then GET /
# with the session cookie.
#
# Everything it starts is removed in a trap, whatever happens.
set -euo pipefail
cd "$(dirname "$0")/.."

run_id="$$"
image="two-tickets-web-e2e:${run_id}"
network="two-tickets-e2e-${run_id}"
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
  echo "web e2e: $*" >&2
  exit 1
}

# The public name a browser would use. It resolves, inside the web
# container, to the fake api, so a call to it is logged rather than lost.
public_name="tickets.public.test"
allowed_origin="http://tickets.lan.test"
password="sentinelsentinelsentinel"

echo "==> web e2e: building the production image and the api binary"
docker build -q -f deploy/web.Dockerfile -t "$image" web/ >/dev/null
CGO_ENABLED=0 go build -o "$work/two-tickets" ./cmd/two-tickets
password_hash="$(printf '%s' "$password" | go run ./deploy/e2e/hashpassword)"

docker network create "$network" >/dev/null
subnet="$(docker network inspect -f '{{(index .IPAM.Config 0).Subnet}}' "$network")"

# start_web <name> <api url> [docker run options...]: starts the image with
# API_INTERNAL_URL set at run time and prints the published base URL. It runs
# in a command substitution, so the caller registers the name for cleanup.
start_web() {
  local name="$1" api_url="$2"
  shift 2
  docker run -d --name "$name" --read-only --tmpfs /tmp \
    --network "$network" -p '127.0.0.1::3000' \
    -e PORT=3000 -e HOSTNAME=0.0.0.0 -e "API_INTERNAL_URL=${api_url}" \
    "$@" "$image" >/dev/null
  local port
  port="$(docker port "$name" 3000 | head -n 1 | sed 's/.*://')"
  local base="http://127.0.0.1:${port}"
  local _
  for _ in $(seq 1 60); do
    if curl -sSf -o /dev/null "${base}/favicon.ico" 2>/dev/null; then
      printf '%s\n' "$base"
      return 0
    fi
    sleep 1
  done
  docker logs "$name" >&2 || true
  fail "web did not become ready"
}

# status_of <curl args...>: prints only the HTTP status.
status_of() {
  curl -sS -o /dev/null -w '%{http_code}' "$@"
}

echo "==> web e2e: part one, server-side rendering against a fake api"
fake="two-tickets-e2e-fake-${run_id}"
containers+=("$fake")
docker run -d --name "$fake" --network "$network" --network-alias fake-api \
  -v "$PWD/deploy/e2e/fake-api.mjs:/e2e/fake-api.mjs:ro" \
  -v "$PWD/testdata/contract:/e2e/contract:ro" \
  --entrypoint node "$image" /e2e/fake-api.mjs >/dev/null
fake_ip="$(docker inspect -f "{{(index .NetworkSettings.Networks \"${network}\").IPAddress}}" "$fake")"

# HOST names the public name too: Seerr's server-side calls went to
# HOST:PORT, and must not any more.
web="two-tickets-e2e-web-ssr-${run_id}"
containers+=("$web")
base="$(start_web "$web" "http://fake-api:8080" -e "HOST=${public_name}" --add-host "${public_name}:${fake_ip}")"

code="$(status_of -H "Host: ${public_name}" -H 'Cookie: two_tickets_session=fake' "${base}/")"
[ "$code" = "200" ] || fail "GET / rendered ${code}, want 200"

encoded_query='/api/v1/search?query=%C3%A9%20a%2Bb%26c%3Dd&page=1&x=%2F%3F+y'
code="$(status_of -H "Host: ${public_name}" "${base}${encoded_query}")"
[ "$code" = "200" ] || fail "GET ${encoded_query} through the proxy = ${code}"

cookie_count="$(curl -sS -D - -o /dev/null "${base}/api/v1/two-cookies" | grep -ci '^set-cookie:' || true)"
[ "$cookie_count" = "2" ] || fail "the proxy returned ${cookie_count} Set-Cookie lines, want 2"

log="$(docker logs "$fake" 2>&1)"
printf '%s\n' "$log" | sed 's/^/web e2e: fake api saw /'
expect_logged() {
  printf '%s\n' "$log" | grep -qF "$1" || fail "the fake api never received: $1"
}
expect_logged '"port":8080,"host":"fake-api:8080","method":"GET","url":"/api/v1/settings/public"'
expect_logged '"port":8080,"host":"fake-api:8080","method":"GET","url":"/api/v1/auth/me","cookie":"two_tickets_session=fake"'
expect_logged "\"port\":8080,\"host\":\"fake-api:8080\",\"method\":\"GET\",\"url\":\"${encoded_query}\""
if printf '%s\n' "$log" | grep -E '"port":(80|3000),|"host":"[^"]*public' >&2; then
  fail "a request reached the public name or web's own port instead of API_INTERNAL_URL"
fi
echo "web e2e: server-side rendering called API_INTERNAL_URL only"

echo "==> web e2e: part two, logging in against the real api"
api="two-tickets-e2e-api-${run_id}"
containers+=("$api")
docker run -d --name "$api" --network "$network" --network-alias api \
  --tmpfs /data:mode=1777 \
  -v "$work/two-tickets:/e2e/two-tickets:ro" \
  -e DATA_DIR=/data -e LISTEN_ADDR=:8080 \
  -e TMDB_API_KEY=sentinelsentinelsentinel \
  -e ACCOUNT_1_KEY=first -e ACCOUNT_1_USERNAME=alice \
  -e ACCOUNT_1_EMAIL=alice@example.com -e ACCOUNT_1_DISPLAY_NAME=Alice \
  -e "ACCOUNT_1_PASSWORD_HASH=${password_hash}" \
  -e ACCOUNT_2_KEY=second -e ACCOUNT_2_USERNAME=bob \
  -e ACCOUNT_2_EMAIL=bob@example.com -e ACCOUNT_2_DISPLAY_NAME=Bob \
  -e "ACCOUNT_2_PASSWORD_HASH=${password_hash}" \
  -e "ALLOWED_ORIGINS=${allowed_origin}" \
  -e "TRUSTED_PROXY_CIDRS=${subnet}" \
  --entrypoint /e2e/two-tickets "$image" >/dev/null

docker rm -f "$web" >/dev/null
web="two-tickets-e2e-web-login-${run_id}"
containers+=("$web")
base="$(start_web "$web" "http://api:8080")"

login_page="$work/login.html"
code=""
for _ in $(seq 1 30); do
  code="$(curl -sS -o "$login_page" -w '%{http_code}' "${base}/login" || true)"
  [ "$code" = "200" ] && break
  sleep 1
done
if [ "$code" != "200" ]; then
  docker logs "$api" >&2 || true
  fail "GET /login = ${code}, want 200"
fi
grep -qF 'Two Tickets' "$login_page" || fail "the login page does not carry the application title"
if grep -qF 'Language Picker' "$login_page"; then
  fail "the login page still shows the language picker"
fi
echo "web e2e: 200 /login"

code="$(status_of "${base}/")"
location="$(curl -sS -o /dev/null -w '%{redirect_url}' "${base}/")"
[ "$code" = "307" ] && [ "${location%/login}" != "$location" ] ||
  fail "GET / without a session = ${code} to '${location}', want 307 to /login"
echo "web e2e: GET / without a session redirects to /login"

login_body="{\"email\":\"alice\",\"password\":\"${password}\"}"
code="$(status_of -X POST -H 'Content-Type: application/json' \
  -H 'Origin: http://elsewhere.test' --data "$login_body" "${base}/api/v1/auth/local")"
[ "$code" = "403" ] || fail "login from a foreign Origin = ${code}, want 403 from the api"
echo "web e2e: login from a foreign Origin is refused with 403"

headers="$work/login.headers"
code="$(curl -sS -o /dev/null -D "$headers" -w '%{http_code}' -X POST \
  -H 'Content-Type: application/json' -H "Origin: ${allowed_origin}" \
  --data "$login_body" "${base}/api/v1/auth/local")"
[ "$code" = "200" ] || fail "login through the proxy = ${code}, want 200"
session_line="$(grep -i '^set-cookie: two_tickets_session=' "$headers" || true)"
[ -n "$session_line" ] || fail "login set no session cookie"
if printf '%s' "$session_line" | grep -qi 'secure'; then
  fail "the session cookie is Secure over plain HTTP: ${session_line}"
fi
session="$(printf '%s' "$session_line" | sed -E 's/^[^:]*: *//; s/;.*//' | tr -d '\r')"
echo "web e2e: logged in through the proxy, cookie set without Secure"

code="$(status_of -H "Cookie: ${session}" "${base}/")"
[ "$code" = "200" ] || fail "GET / with the session = ${code}, want 200"
me="$(curl -sS -H "Cookie: ${session}" "${base}/api/v1/auth/me")"
printf '%s' "$me" | grep -qF '"username":"alice"' || fail "auth/me through the proxy = ${me}"
echo "web e2e: GET / with the session renders as alice"

echo "web e2e: green"
