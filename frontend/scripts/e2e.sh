#!/usr/bin/env bash
# Run the e2e suite against a Wails dev server, keeping the server WARM between
# runs. The cold `wails dev` startup (go mod tidy + npm install + full CGO/
# WebKit build) is ~4 minutes; the 16 tests themselves take under a minute, so
# the build is amortized: first run pays it, every later run reuses the server
# (playwright.config.ts has reuseExistingServer outside CI).
#
#   npm run e2e                 # ensure server, run suite, leave server warm
#   npm run e2e -- e2e/manual.spec.ts   # pass args through to playwright
#   npm run e2e:stop            # stop the dev server
#
# CI has no reuse benefit, so there the server is stopped after the suite.
# When the script starts the server it kills the whole process group — a plain
# child kill leaves wails' `vite` grandchild alive holding the output pipes,
# which makes any waiting caller hang forever.
set -uo pipefail

PORT=34115
LOG=/tmp/aelum-e2e-wails-dev.log
cd "$(dirname "$0")/.."

up() { curl -sf -o /dev/null "http://localhost:${PORT}"; }

stop() {
	local pid
	pid=$(cat "/tmp/aelum-e2e-wails-dev.pid" 2>/dev/null) || true
	if [[ -n "${pid:-}" ]]; then
		kill -- "-${pid}" 2>/dev/null || kill "${pid}" 2>/dev/null
	fi
	rm -f /tmp/aelum-e2e-wails-dev.pid
}

if [[ "${1:-}" == "--stop" ]]; then
	stop
	exit 0
fi

if up; then
	exec npx playwright test "$@"
fi

if [[ -n "${CI:-}" ]]; then
	# Playwright starts and stops its own webServer on CI.
	exec npx playwright test "$@"
fi

# The launcher must run in the frontend dir (playwright.config.ts lives there);
# wails dev itself must run at the wails.json project root (the repo root,
# one level above frontend/ — the top of this script already cd'd to frontend).
FE_DIR=$(pwd)

# .. is the wails.json project root.
cd ..
env AELUM_DRY_RUN=1 FAKE_DISKS=1 setsid wails dev -tags webkit2_41 >"$LOG" 2>&1 &
SRV=$!
echo "$SRV" >/tmp/aelum-e2e-wails-dev.pid

for _ in $(seq 1 300); do
	up && break
	kill -0 "$SRV" 2>/dev/null || { echo "wails dev exited; see $LOG" >&2; exit 1; }
	sleep 1
done
if ! up; then
	echo "wails dev did not serve :${PORT} in 300s; see $LOG" >&2
	kill -- "-${SRV}" 2>/dev/null
	exit 1
fi

cd "$FE_DIR"
npx playwright test "$@"
# The server is intentionally left running: that is the whole point (npm run
# e2e:stop ends it). CI never reaches this path — it execs playwright above.
exit $?
