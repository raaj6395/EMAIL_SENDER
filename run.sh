#!/usr/bin/env bash
#
# run.sh — one-command setup + launch for the Resume Cold-Email Sender + Job Search.
#
# On a fresh laptop this will:
#   1. Check that Go and Node are installed (and new enough).
#   2. Create backend/.env from the template (prompting you to add Gmail creds).
#   3. Create frontend/.env.local pointing at the backend.
#   4. Check that your resume PDF is in place.
#   5. Install/tidy dependencies for both backend and frontend.
#   6. Launch the backend (:8080) and frontend (:3000) together.
#
# Usage:
#   ./run.sh              # full setup + run both servers
#   ./run.sh setup        # setup only (deps, env files) — don't start servers
#   ./run.sh doctor       # just check prerequisites and config, report status
#
# Stop everything with Ctrl-C.

set -euo pipefail

# ── Resolve project directories (works from anywhere) ───────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND_DIR="$SCRIPT_DIR/backend"
FRONTEND_DIR="$SCRIPT_DIR/frontend"

BACKEND_PORT="${BACKEND_PORT:-8080}"
FRONTEND_PORT="${FRONTEND_PORT:-3000}"

# ── Pretty output helpers ───────────────────────────────────────────────────
if [ -t 1 ]; then
  BOLD=$'\033[1m'; DIM=$'\033[2m'; RED=$'\033[31m'; GREEN=$'\033[32m'
  YELLOW=$'\033[33m'; BLUE=$'\033[34m'; RESET=$'\033[0m'
else
  BOLD=""; DIM=""; RED=""; GREEN=""; YELLOW=""; BLUE=""; RESET=""
fi
info()  { printf "%s→%s %s\n" "$BLUE" "$RESET" "$*"; }
ok()    { printf "%s✓%s %s\n" "$GREEN" "$RESET" "$*"; }
warn()  { printf "%s!%s %s\n" "$YELLOW" "$RESET" "$*"; }
err()   { printf "%s✗%s %s\n" "$RED" "$RESET" "$*" >&2; }
step()  { printf "\n%s%s%s\n" "$BOLD" "$*" "$RESET"; }

# ── Version helpers ─────────────────────────────────────────────────────────
# Compare dotted versions: returns 0 if $1 >= $2.
version_ge() {
  [ "$(printf '%s\n%s\n' "$2" "$1" | sort -V | head -n1)" = "$2" ]
}

check_prereqs() {
  step "1. Checking prerequisites"
  local missing=0

  if command -v go >/dev/null 2>&1; then
    local gov
    gov="$(go version | awk '{print $3}' | sed 's/go//')"
    if version_ge "$gov" "1.22"; then
      ok "Go $gov"
    else
      err "Go $gov found, but 1.22+ is required. Update: https://go.dev/dl/"
      missing=1
    fi
  else
    err "Go is not installed. Install it: https://go.dev/dl/  (or: brew install go)"
    missing=1
  fi

  if command -v node >/dev/null 2>&1; then
    local nov
    nov="$(node --version | sed 's/v//')"
    if version_ge "$nov" "18.0.0"; then
      ok "Node $nov"
    else
      err "Node $nov found, but 18+ is required. Update: https://nodejs.org/  (or: brew install node)"
      missing=1
    fi
  else
    err "Node.js is not installed. Install it: https://nodejs.org/  (or: brew install node)"
    missing=1
  fi

  if command -v npm >/dev/null 2>&1; then
    ok "npm $(npm --version)"
  else
    err "npm is not installed (usually comes with Node.js)."
    missing=1
  fi

  if [ "$missing" -ne 0 ]; then
    err "Please install the missing tools above, then re-run ./run.sh"
    exit 1
  fi
}

setup_backend_env() {
  step "2. Backend configuration (backend/.env)"
  local env_file="$BACKEND_DIR/.env"
  local example="$BACKEND_DIR/.env.example"

  if [ -f "$env_file" ]; then
    ok ".env already exists — leaving it as-is"
  else
    if [ ! -f "$example" ]; then
      err "backend/.env.example is missing — cannot create .env"
      exit 1
    fi
    cp "$example" "$env_file"
    ok "Created backend/.env from template"
    warn "You MUST edit backend/.env and set your Gmail credentials:"
    printf "    %sGMAIL_USER%s=you@gmail.com\n" "$DIM" "$RESET"
    printf "    %sGMAIL_APP_PASSWORD%s=<16-char app password>\n" "$DIM" "$RESET"
    printf "  Generate an App Password (needs 2-Step Verification ON):\n"
    printf "    %shttps://myaccount.google.com/apppasswords%s\n" "$DIM" "$RESET"
    printf "  (Optional) add OPENAI_API_KEY for AI-tailored emails + job eligibility.\n"
    printf "  (Optional) add APIFY_TOKEN to enable the LinkedIn lookup + Job Search page.\n"
  fi

  # Warn if the credentials still look like placeholders.
  if grep -qE '^GMAIL_USER=you@gmail\.com' "$env_file" 2>/dev/null \
     || grep -qE '^GMAIL_APP_PASSWORD=(xxx|<)' "$env_file" 2>/dev/null; then
    warn "backend/.env still has placeholder Gmail values — sending won't work until you fill them in."
  fi
}

setup_frontend_env() {
  step "3. Frontend configuration (frontend/.env.local)"
  local env_file="$FRONTEND_DIR/.env.local"
  local url="http://localhost:$BACKEND_PORT"
  if [ -f "$env_file" ]; then
    ok ".env.local already exists"
  else
    printf "NEXT_PUBLIC_API_URL=%s\n" "$url" > "$env_file"
    ok "Created frontend/.env.local (API → $url)"
  fi
}

check_resume() {
  step "4. Resume check (backend/data/resume.pdf)"
  local resume="$BACKEND_DIR/data/resume.pdf"
  mkdir -p "$BACKEND_DIR/data"
  if [ -s "$resume" ]; then
    ok "Resume found ($(du -h "$resume" | awk '{print $1}'))"
  else
    warn "No resume yet. Place your resume PDF here (exact name):"
    printf "    %s%s%s\n" "$DIM" "$resume" "$RESET"
    printf "  The app attaches it to every email. You can add it now or later.\n"
  fi
}

install_backend() {
  step "5. Installing backend dependencies"
  info "Running 'go mod tidy' (downloads Go modules)…"
  ( cd "$BACKEND_DIR" && go mod tidy )
  info "Building backend to verify it compiles…"
  ( cd "$BACKEND_DIR" && go build ./... )
  ok "Backend ready"
}

install_frontend() {
  step "6. Installing frontend dependencies"
  if [ -d "$FRONTEND_DIR/node_modules" ]; then
    ok "node_modules already present — skipping npm install (delete it to force reinstall)"
  else
    info "Running 'npm install' (this can take a minute)…"
    # Use a writable cache dir to avoid permission issues on some setups.
    ( cd "$FRONTEND_DIR" && npm_config_cache="${NPM_CONFIG_CACHE:-$HOME/.npm-emailsender}" npm install )
    ok "Frontend dependencies installed"
  fi
}

# ── Launch both servers, shut both down cleanly on Ctrl-C ───────────────────
BACKEND_PID=""
FRONTEND_PID=""
cleanup() {
  printf "\n"
  info "Shutting down…"
  [ -n "$FRONTEND_PID" ] && kill "$FRONTEND_PID" 2>/dev/null || true
  [ -n "$BACKEND_PID" ]  && kill "$BACKEND_PID"  2>/dev/null || true
  wait 2>/dev/null || true
  ok "Stopped."
}

port_in_use() { lsof -ti:"$1" >/dev/null 2>&1; }

run_servers() {
  step "7. Starting servers"

  if port_in_use "$BACKEND_PORT"; then
    err "Port $BACKEND_PORT is already in use. Stop the process using it, or set BACKEND_PORT=xxxx."
    exit 1
  fi
  if port_in_use "$FRONTEND_PORT"; then
    err "Port $FRONTEND_PORT is already in use. Stop the process using it, or set FRONTEND_PORT=xxxx."
    exit 1
  fi

  trap cleanup INT TERM EXIT

  info "Starting backend on http://localhost:$BACKEND_PORT …"
  ( cd "$BACKEND_DIR" && PORT="$BACKEND_PORT" go run . ) &
  BACKEND_PID=$!

  # Wait for the backend health endpoint to respond.
  local up=0
  for _ in $(seq 1 40); do
    if curl -sf "http://localhost:$BACKEND_PORT/api/health" >/dev/null 2>&1; then up=1; break; fi
    # If the backend process died early, stop waiting.
    if ! kill -0 "$BACKEND_PID" 2>/dev/null; then break; fi
    sleep 0.5
  done
  if [ "$up" -eq 1 ]; then
    ok "Backend is up"
  else
    err "Backend did not become healthy — check the log output above."
    exit 1
  fi

  info "Starting frontend on http://localhost:$FRONTEND_PORT …"
  ( cd "$FRONTEND_DIR" && npm_config_cache="${NPM_CONFIG_CACHE:-$HOME/.npm-emailsender}" npm run dev -- --port "$FRONTEND_PORT" ) &
  FRONTEND_PID=$!

  printf "\n%s%s========================================%s\n" "$BOLD" "$GREEN" "$RESET"
  printf "  %sReady!%s Open %shttp://localhost:%s%s in your browser.\n" "$BOLD" "$RESET" "$BOLD" "$FRONTEND_PORT" "$RESET"
  printf "    • Email Sender: http://localhost:%s/\n" "$FRONTEND_PORT"
  printf "    • Job Search:   http://localhost:%s/jobs  (needs APIFY_TOKEN)\n" "$FRONTEND_PORT"
  printf "  Backend API: http://localhost:%s\n" "$BACKEND_PORT"
  printf "  Press %sCtrl-C%s to stop both servers.\n" "$BOLD" "$RESET"
  printf "%s%s========================================%s\n\n" "$BOLD" "$GREEN" "$RESET"

  # Wait on both; if either exits, cleanup() runs via the EXIT trap.
  wait
}

# ── Entry point ─────────────────────────────────────────────────────────────
MODE="${1:-run}"

case "$MODE" in
  doctor)
    check_prereqs
    setup_backend_env
    setup_frontend_env
    check_resume
    step "Doctor complete"
    ok "Prerequisites and config checked. Run './run.sh' to install deps and start."
    ;;
  setup)
    check_prereqs
    setup_backend_env
    setup_frontend_env
    check_resume
    install_backend
    install_frontend
    step "Setup complete"
    ok "Everything installed. Run './run.sh' to start the app."
    ;;
  run|"")
    check_prereqs
    setup_backend_env
    setup_frontend_env
    check_resume
    install_backend
    install_frontend
    run_servers
    ;;
  *)
    err "Unknown command: $MODE"
    printf "Usage: ./run.sh [run|setup|doctor]\n"
    exit 1
    ;;
esac
