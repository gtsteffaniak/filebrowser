#!/usr/bin/env bash
# Run one Playwright suite directly on the runner (CI and local).
#
# Replaces the per-suite _docker/Dockerfile.playwright-<suite> images: stages a
# scratch working tree at .pw/<suite>/ mirroring the container's /app layout
# (backend + frontend siblings), copies the suite's backend config files and
# frontend playwright.config.ts from _docker/src/<suite>/, starts filebrowser —
# plus nginx for proxy/jwt and the mock OIDC server for oidc — waits for the
# server, then runs `npx playwright test` from frontend/.
#
# Prereqs on the runner: backend/filebrowser, backend/internal/web/dist,
# frontend/node_modules and the chromium browser must already exist
# (the prepare job in pr.yaml produces all of them).
#
#   _docker/run-playwright-suite.sh general
set -euo pipefail

SUITE="${1:?usage: run-playwright-suite.sh <suite> (general|settings|noauth|sharing|no-config|previews|screenshots|proxy|jwt|oidc|performance)}"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SRC="${ROOT}/_docker/src/${SUITE}"
RUN="${ROOT}/.pw/${SUITE}"
BACKEND_DIR="${RUN}/backend"
FRONTEND="${ROOT}/frontend"
LOG="${PLAYWRIGHT_SERVER_LOG:-${ROOT}/server.log}"
HEALTH_TIMEOUT="${HEALTH_TIMEOUT:-60}"

die() { echo "run-playwright-suite: $*" >&2; exit 1; }

[[ -d "${SRC}" ]] || die "unknown suite '${SUITE}' (no ${SRC})"
[[ -x "${ROOT}/backend/filebrowser" ]] || die "missing backend/filebrowser binary"
[[ -d "${ROOT}/backend/internal/web/dist" ]] || die "missing backend/internal/web/dist (frontend build)"

PIDS=()
NGINX_STARTED=0
SAVED_CONFIG=""

cleanup() {
  set +e
  for pid in "${PIDS[@]:-}"; do
    kill "${pid}" 2>/dev/null
  done
  [[ ${#PIDS[@]:-0} -gt 0 ]] && wait "${PIDS[@]}" 2>/dev/null
  if [[ "${NGINX_STARTED}" == "1" ]]; then
    sudo nginx -s stop >/dev/null 2>&1
  fi
  # Restore the repo's frontend/playwright.config.ts if a suite config overlaid it.
  if [[ -n "${SAVED_CONFIG}" && -f "${SAVED_CONFIG}" ]]; then
    mv "${SAVED_CONFIG}" "${FRONTEND}/playwright.config.ts"
  fi
}
trap cleanup EXIT INT TERM

# --- stage the working tree ---------------------------------------------------
rm -rf "${RUN}"
mkdir -p "${BACKEND_DIR}/internal/web" "${RUN}/frontend/tests"
cp "${ROOT}/backend/filebrowser" "${BACKEND_DIR}/filebrowser"
chmod +x "${BACKEND_DIR}/filebrowser"
cp -a "${ROOT}/backend/internal/web/dist" "${BACKEND_DIR}/internal/web/dist"
# Sources reference ../frontend/tests/playwright-files relative to the backend cwd;
# copy (not symlink) so test mutations stay inside the scratch tree.
cp -a "${FRONTEND}/tests/playwright-files" "${RUN}/frontend/tests/playwright-files"
if [[ -d "${SRC}/backend" ]]; then
  cp -a "${SRC}/backend/." "${BACKEND_DIR}/"
fi
if [[ -f "${SRC}/frontend/playwright.config.ts" ]]; then
  SAVED_CONFIG="${FRONTEND}/playwright.config.ts.orig"
  cp "${FRONTEND}/playwright.config.ts" "${SAVED_CONFIG}"
  cp "${SRC}/frontend/playwright.config.ts" "${FRONTEND}/playwright.config.ts"
fi
# Extra frontend files (e.g. mock-oidc-server.js for oidc).
if [[ -d "${SRC}/frontend" ]]; then
  find "${SRC}/frontend" -mindepth 1 -maxdepth 1 ! -name 'playwright.config.ts' -exec cp -a {} "${FRONTEND}/" \;
fi

# Binding to port 80 from a non-root user needs CAP_NET_BIND_SERVICE.
need_priv_port() {
  command -v setcap >/dev/null || { sudo apt-get update -qq && sudo apt-get install -y libcap2-bin; }
  sudo setcap 'cap_net_bind_service=+ep' "${BACKEND_DIR}/filebrowser"
}

fb() { (cd "${BACKEND_DIR}" && ./filebrowser "$@"); }

start_fb() {
  if command -v stdbuf >/dev/null; then
    (cd "${BACKEND_DIR}" && stdbuf -oL -eL env FILEBROWSER_PLAYWRIGHT_TEST=true "$@" ./filebrowser) >>"${LOG}" 2>&1 &
  else
    (cd "${BACKEND_DIR}" && env FILEBROWSER_PLAYWRIGHT_TEST=true "$@" ./filebrowser) >>"${LOG}" 2>&1 &
  fi
  PIDS+=($!)
}

wait_for() {
  local url="$1" deadline=$(( $(date +%s) + HEALTH_TIMEOUT )) n=0
  until curl -s -o /dev/null --max-time 2 "${url}"; do
    if ! kill -0 "${PIDS[-1]}" 2>/dev/null; then
      echo "filebrowser exited before serving (attempt ${n})" >&2
      tail -80 "${LOG}" >&2 || true
      exit 1
    fi
    if [[ "$(date +%s)" -ge "${deadline}" ]]; then
      echo "server did not come up at ${url} after ${HEALTH_TIMEOUT}s" >&2
      tail -80 "${LOG}" >&2 || true
      exit 1
    fi
    n=$((n + 1))
    sleep 1
  done
  echo "server up at ${url} after ${n}s" >&2
}

setup_nginx() {
  command -v nginx >/dev/null || { sudo apt-get update -qq && sudo apt-get install -y nginx; }
  sudo rm -f /etc/nginx/sites-enabled/default
  sed 's/filebrowser/localhost/g' "${BACKEND_DIR}/default.conf" | sudo tee /etc/nginx/conf.d/default.conf >/dev/null
  if [[ -f "${BACKEND_DIR}/.htpasswd" ]]; then
    sudo cp "${BACKEND_DIR}/.htpasswd" /etc/nginx/conf.d/.htpasswd
  fi
  sudo nginx
  NGINX_STARTED=1
}

PW_ARGS=()

: >"${LOG}"
case "${SUITE}" in
  general)
    cp "${ROOT}/backend/reduce-rounded-corners.css" "${BACKEND_DIR}/no-rounded.css"
    dd if=/dev/zero of="${RUN}/frontend/tests/playwright-files/1.1MB.bin" bs=1024 count=1126 status=none
    dd if=/dev/zero of="${RUN}/frontend/tests/playwright-files/1.1mb.bin" bs=1024 count=1126 status=none
    fb user set testuser1 --password testuser1
    need_priv_port
    start_fb
    wait_for "http://127.0.0.1/health"
    ;;
  settings)
    cp "${ROOT}/backend/reduce-rounded-corners.css" "${BACKEND_DIR}/no-rounded.css"
    fb user set testuser1 --password testuser1
    fb set rule -s access -p / -r user -v admin --allow -c config.yaml
    fb set rule -s access -p /excluded/showme.txt -r user -v admin --allow -c config.yaml
    fb set rule -s access -p /denied -r user -v admin -c config.yaml
    fb set rule -s access -p /excluded -r user -v admin -c config.yaml
    sudo mkdir -p /tests/playwright-files
    sudo cp -a "${RUN}/frontend/tests/playwright-files/." /tests/playwright-files/
    sudo chmod -R a+rwX /tests/playwright-files
    need_priv_port
    start_fb
    wait_for "http://127.0.0.1/health"
    ;;
  noauth)
    cp -a "${RUN}/frontend/tests/playwright-files" "${RUN}/frontend/tests/playwright-files2"
    need_priv_port
    start_fb
    wait_for "http://127.0.0.1/health"
    ;;
  sharing|previews|oidc)
    need_priv_port
    if [[ "${SUITE}" == "oidc" ]]; then
      (cd "${FRONTEND}" && node mock-oidc-server.js) >>"${LOG}" 2>&1 &
      PIDS+=($!)
      sleep 2
    fi
    start_fb
    wait_for "http://127.0.0.1/health"
    ;;
  no-config)
    need_priv_port
    start_fb FILEBROWSER_ADMIN_PASSWORD=playwright-password
    wait_for "http://127.0.0.1/health"
    ;;
  screenshots)
    start_fb
    wait_for "http://127.0.0.1:8080/health"
    PW_ARGS+=(--project dark-screenshots)
    ;;
  proxy)
    setup_nginx
    start_fb
    wait_for "http://127.0.0.1:8080/health"
    ;;
  jwt)
    setup_nginx
    start_fb
    wait_for "http://127.0.0.1:8080/health"
    ;;
  performance)
    cp "${ROOT}/backend/reduce-rounded-corners.css" "${BACKEND_DIR}/no-rounded.css"
    need_priv_port
    export CI=true
    export PERF_BASE_URL="http://127.0.0.1/"
    export PERF_SCALES="${PERF_SCALES:-100,1000,10000}"
    export PERF_WORKERS="${PERF_WORKERS:-6}"
    export PERF_TEST_TIMEOUT_MS="${PERF_TEST_TIMEOUT_MS:-480000}"
    export PERF_REPEATS="${PERF_REPEATS:-1}"
    export PERF_BROWSERS="${PERF_BROWSERS:-chromium}"
    export PERF_IMAGE_TAG="${PERF_IMAGE_TAG:-playwright-base-chromium}"
    export PERF_IN_CONTAINER=0
    start_fb
    wait_for "http://127.0.0.1/health"
    PW_ARGS+=(-c playwright.performance.config.ts)
    ;;
esac

cd "${FRONTEND}"
echo "starting playwright suite=${SUITE} args=${PW_ARGS[*]:-none}" >&2
set +e
npx playwright test "${PW_ARGS[@]}"
rc=$?
set -e

if [[ "${rc}" -ne 0 ]]; then
  echo "=== tail of server.log (last 200 lines) ===" >&2
  tail -200 "${LOG}" >&2 || true
fi
echo "playwright exited rc=${rc}" >&2
exit "${rc}"
