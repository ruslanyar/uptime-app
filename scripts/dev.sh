#!/usr/bin/env bash
# Local development only. No user environment files are sourced or rewritten.
set -euo pipefail
# Each managed command gets its own process group, including its descendants.
set -m

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKEND="$ROOT/backend"
FRONTEND="$ROOT/frontend"
STATE="$ROOT/.dev"
LOCK="$STATE/launcher.lock"
pids=()
pid_count=0
last_pid=""
lock_owned=false
db_was_running=true
db_start_requested=false

info() { printf '[dev] %s\n' "$*"; }
fail() { printf '[dev] Ошибка: %s\n' "$*" >&2; exit 1; }
prefix() {
  local label="$1" line
  while IFS= read -r line || [[ -n "$line" ]]; do printf '[%s] %s\n' "$label" "$line"; done
}

cleanup() {
  local code=$? pid remaining deadline
  trap - EXIT
  trap '' INT TERM
  if [[ "$pid_count" -gt 0 ]]; then
    info 'Останавливаем приложения…'
    for pid in "${pids[@]}"; do kill -TERM -- "-$pid" 2>/dev/null || true; done
    deadline=$((SECONDS + 15))
    while (( SECONDS < deadline )); do
      remaining=false
      for pid in "${pids[@]}"; do
        if kill -0 -- "-$pid" 2>/dev/null; then remaining=true; fi
      done
      [[ "$remaining" == true ]] || break
      sleep 1
    done
    for pid in "${pids[@]}"; do
      kill -KILL -- "-$pid" 2>/dev/null || true
      wait "$pid" 2>/dev/null || true
    done
  fi
  if [[ "$db_start_requested" == true && "$db_was_running" == false ]]; then
    info 'Останавливаем запущенную скриптом PostgreSQL; данные сохраняются.'
    (cd "$BACKEND" && docker compose -f compose.yaml stop -t 10 postgres) || code=1
  fi
  if [[ "$lock_owned" == true ]]; then
    rm -f "$STATE/jwt-secret.tmp" "$LOCK/pid"
    rmdir "$LOCK" || code=1
  fi
  exit "$code"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

start() {
  local label="$1" directory="$2"
  shift 2
  (cd "$directory" && exec "$@") > >(prefix "$label") 2>&1 &
  last_pid=$!
  pids+=("$last_pid")
  pid_count=$((pid_count + 1))
}
run() {
  local label="$1" pid code index
  start "$@"
  pid="$last_pid"
  if wait "$pid"; then code=0; else code=$?; fi
  [[ "$code" == 0 ]] || fail "Этап '$label' завершился с кодом $code."
  for index in "${!pids[@]}"; do
    if [[ "${pids[$index]}" == "$pid" ]]; then
      unset 'pids[index]'
      pid_count=$((pid_count - 1))
    fi
  done
}
ensure_running() {
  kill -0 "$api_pid" 2>/dev/null || fail 'API неожиданно завершился.'
  if [[ -n "${frontend_pid:-}" ]]; then
    kill -0 "$frontend_pid" 2>/dev/null || fail 'Фронтенд неожиданно завершился.'
  fi
}
ready() {
  local url="$1" expected="$2" label="$3" deadline status
  deadline=$((SECONDS + 60))
  while (( SECONDS < deadline )); do
    ensure_running
    status="$(curl --noproxy '*' -s -o /dev/null -w '%{http_code}' --connect-timeout 1 --max-time 2 "$url" || true)"
    if [[ "$status" == "$expected" ]]; then return; fi
    sleep 1
  done
  fail "$label не готов за 60 секунд ($url)."
}

for tool in node npm go docker curl openssl; do
  command -v "$tool" >/dev/null 2>&1 || fail "Не найдена команда $tool. См. README."
done
node -e 'const [major, minor] = process.versions.node.split(".").map(Number); process.exit((major === 20 && minor >= 19) || (major === 22 && minor >= 12) || major >= 24 ? 0 : 1)' || fail 'Нужен Node.js 20.19+, 22.12+ или 24+.'
go_version="$(go env GOVERSION)"
[[ "$go_version" =~ ^go([0-9]+)\.([0-9]+) ]] || fail 'Не удалось определить версию Go.'
(( BASH_REMATCH[1] > 1 || (BASH_REMATCH[1] == 1 && BASH_REMATCH[2] >= 27) )) || fail 'Нужен Go 1.27+.'
docker compose version >/dev/null || fail 'Нужен Docker Compose v2.'
docker info >/dev/null 2>&1 || fail 'Docker daemon недоступен. Запустите Docker.'

umask 077
mkdir -p "$STATE"
chmod 700 "$STATE"
if ! mkdir "$LOCK" 2>/dev/null; then
  [[ -f "$LOCK/pid" ]] || fail 'Другой экземпляр готовится к запуску; повторите позже.'
  owner="$(cat "$LOCK/pid")"
  [[ "$owner" =~ ^[0-9]+$ ]] || fail 'Некорректная блокировка .dev/launcher.lock; проверьте её вручную.'
  if kill -0 "$owner" 2>/dev/null; then fail "Окружение уже запускается или работает (PID $owner)."; fi
  rm -f "$LOCK/pid"
  rmdir "$LOCK" || fail 'Не удалось удалить устаревшую блокировку.'
  mkdir "$LOCK" || fail 'Другой экземпляр уже получил блокировку.'
fi
lock_owned=true
printf '%s\n' "$$" > "$LOCK/pid"

# Bind only to loopback; never terminate a process already using these ports.
node - <<'JS' || fail 'Порт 3000 или 8080 занят; освободите его и повторите запуск.'
const net = require("node:net");
(async () => {
  for (const port of [3000, 8080]) {
    await new Promise((resolve, reject) => {
      const server = net.createServer();
      server.once("error", () => reject(new Error(`Порт ${port} недоступен`)));
      server.listen(port, "127.0.0.1", () => server.close(resolve));
    });
  }
})().catch((error) => { console.error(`[dev] ${error.message}`); process.exitCode = 1; });
JS

fingerprint="$(cat "$FRONTEND/package.json" "$FRONTEND/package-lock.json" | openssl dgst -sha256)"
if [[ ! -d "$FRONTEND/node_modules" || ! -f "$STATE/frontend-fingerprint" || "$(cat "$STATE/frontend-fingerprint")" != "$fingerprint" ]]; then
  info 'Устанавливаем зависимости фронтенда…'
  run npm "$FRONTEND" npm ci
  printf '%s\n' "$fingerprint" > "$STATE/frontend-fingerprint"
fi

if [[ ! -f "$STATE/jwt-secret" ]]; then
  openssl rand -hex 32 > "$STATE/jwt-secret.tmp"
  mv "$STATE/jwt-secret.tmp" "$STATE/jwt-secret"
fi
chmod 600 "$STATE/jwt-secret"
JWT_SECRET="$(cat "$STATE/jwt-secret")"
[[ "$JWT_SECRET" =~ ^[0-9a-f]{64}$ ]] || fail 'Некорректный локальный секрет .dev/jwt-secret.'
export JWT_SECRET
export DATABASE_URL='postgres://uptime:local-development-only@localhost:5432/uptime?sslmode=disable'
export HTTP_ADDR='127.0.0.1:8080'
export JWT_ISSUER='uptime-api'
export JWT_AUDIENCE='uptime-client'
export ALLOWED_ORIGINS='http://localhost:3000'
export COOKIE_SECURE=false
export COOKIE_SAME_SITE=lax

info 'Собираем API и мигратор…'
run build "$BACKEND" go build -o "$STATE/api" ./cmd/api
run build "$BACKEND" go build -o "$STATE/migrate" ./cmd/migrate
if [[ -n "$(cd "$BACKEND" && docker compose -f compose.yaml ps --status running -q postgres)" ]]; then db_was_running=true; else db_was_running=false; fi
db_start_requested=true
run postgres "$BACKEND" docker compose -f compose.yaml up -d --wait --wait-timeout 60 postgres
run migrate "$BACKEND" "$STATE/migrate" up

start api "$BACKEND" "$STATE/api"
api_pid="$last_pid"
ready 'http://127.0.0.1:8080/api/v1/auth/me' 401 API
# The public origin overrides .env.local without changing it. Use the normal Next directory.
start frontend "$FRONTEND" env NEXT_PUBLIC_API_URL='http://localhost:8080' E2E_DIST_DIR='.next' LOCAL_DEV_LAUNCH=1 node "$FRONTEND/node_modules/next/dist/bin/next" dev --hostname 127.0.0.1 --port 3000
frontend_pid="$last_pid"
ready 'http://127.0.0.1:3000/login' 307 'Перенаправление на localhost'
ready 'http://localhost:3000/login' 200 'Фронтенд'
info 'Окружение готово. Фронтенд: http://localhost:3000 | API: http://localhost:8080'
info 'Ctrl+C остановит приложения. Аккаунты и сессии сохраняются между запусками.'
while true; do ensure_running; sleep 1; done
