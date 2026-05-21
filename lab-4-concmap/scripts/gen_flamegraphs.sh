#!/usr/bin/env bash
set -euo pipefail

PROF_DIR="${1:-metrics/profiles}"
PLOT_DIR="${2:-metrics/plots}"
PORT=18092

mkdir -p "$PLOT_DIR"

profiles=(
  cpu_parallel_get_conc
  cpu_parallel_get_plain
  mem_parallel_get_conc
  mem_parallel_get_plain
)

screenshot_flame_png() {
  local html="$1"
  local png="$2"
  local dir base url
  dir="$(cd "$(dirname "$html")" && pwd)"
  base="$(basename "$html")"
  url="file://${dir}/${base}"
  [[ -f "$html" ]] || return 1
  for c in google-chrome chromium chromium-browser; do
    if command -v "$c" >/dev/null 2>&1; then
      if "$c" --headless=new --no-sandbox --disable-gpu \
        --screenshot="$png" --window-size="1800,1000" \
        "$url" 2>/dev/null; then
        echo "    PNG: $(basename "$png")"
        return 0
      fi
      if "$c" --headless --no-sandbox --disable-gpu \
        --screenshot="$png" --window-size="1800,1000" \
        "$url" 2>/dev/null; then
        echo "    PNG: $(basename "$png")"
        return 0
      fi
    fi
  done
  return 1
}

for prof in "${profiles[@]}"; do
  f="${PROF_DIR}/${prof}.prof"
  if [[ ! -f "$f" ]]; then
    echo "skip flamegraph (${prof}: нет профиля)"
    continue
  fi
  echo "→ flamegraph: ${prof}"

  fuser -k "${PORT}/tcp" 2>/dev/null || true
  sleep 0.5

  go tool pprof -http=":${PORT}" "$f" &
  PPROF_PID=$!
  sleep 2

  out_html="${PLOT_DIR}/flamegraph_${prof}.html"
  curl -sf "http://localhost:${PORT}/ui/flamegraph" -o "${out_html}" || true

  kill "$PPROF_PID" 2>/dev/null || true
  wait "$PPROF_PID" 2>/dev/null || true
  sleep 0.5

  if screenshot_flame_png "${out_html}" "${PLOT_DIR}/flamegraph_${prof}.png"; then
    :
  else
    echo "    (нет headless chromium — только HTML flamegraph)"
  fi
done
