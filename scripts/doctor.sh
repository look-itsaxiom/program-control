#!/bin/sh
# Reports whether the toolchain needed for program-control is installed. Needs nothing but a POSIX shell.

missing_required=""

add_missing() {
  if [ -z "$missing_required" ]; then
    missing_required="$1"
  else
    missing_required="$missing_required, $1"
  fi
}

version_ge() {
  [ "$1" = "$2" ] && return 0
  higher=$(printf '%s\n%s\n' "$1" "$2" | sort -V | tail -n1)
  [ "$higher" = "$1" ]
}

get_version() {
  case "$1" in
    git) git --version 2>/dev/null | sed 's/git version //' ;;
    go) go version 2>/dev/null | sed 's/^go version go//' | sed 's/ .*$//' ;;
    node) node --version 2>/dev/null | sed 's/^v//' ;;
    pnpm) pnpm --version 2>/dev/null ;;
    docker) docker version --format '{{.Client.Version}}' 2>/dev/null ;;
    gh) gh --version 2>/dev/null | head -n1 | sed 's/^gh version //' | sed 's/ .*$//' ;;
    claude) claude --version 2>/dev/null | head -n1 ;;
  esac
}

print_row() {
  printf '%-14s %-8s %-14s %-6s %s\n' "$1" "$2" "$3" "$4" "$5"
}

check() {
  name=$1; required=$2; cmd=$3; minver=$4; url=$5
  if ! command -v "$cmd" >/dev/null 2>&1; then
    print_row "$name" "MISSING" "" "$minver" "$url"
    [ "$required" = "true" ] && add_missing "$name"
    return
  fi
  ver=$(get_version "$cmd")
  status="ok"
  clean_ver=$(printf '%s' "$ver" | sed 's/[^0-9.].*$//' | sed 's/\.*$//')
  if [ -n "$minver" ] && [ -n "$clean_ver" ]; then
    if ! version_ge "$clean_ver" "$minver"; then
      status="TOO OLD"
    fi
  fi
  print_row "$name" "$status" "$ver" "$minver" "$url"
  if [ "$required" = "true" ] && [ "$status" != "ok" ]; then
    add_missing "$name"
  fi
}

print_row "Tool" "Status" "Version" "Need" "Get"
check git    true  git    2.30 https://git-scm.com/downloads
check go     true  go     1.22 https://go.dev/dl/
check node   true  node   22.0 https://nodejs.org/
check pnpm   true  pnpm   10.0 https://pnpm.io/installation
check docker true  docker 24.0 https://www.docker.com/products/docker-desktop/
check gh     false gh     2.40 https://cli.github.com/
check claude false claude ""   https://docs.claude.com/en/docs/claude-code/overview

# Docker daemon and compose v2
if docker info --format '{{.ServerVersion}}' >/dev/null 2>&1; then
  daemon_status=ok
else
  daemon_status=MISSING
  add_missing "docker daemon"
fi
print_row "docker daemon" "$daemon_status" "" "running" "Start Docker Desktop"

compose_version=$(docker compose version --short 2>/dev/null)
compose_rc=$?
if [ "$compose_rc" -eq 0 ] && [ -n "$compose_version" ]; then
  compose_status=ok
else
  compose_status=MISSING
  add_missing "docker compose"
fi
print_row "docker compose" "$compose_status" "$compose_version" "v2" "Included with Docker Desktop"

if [ -n "$missing_required" ]; then
  printf '\nRequired tools missing or too old: %s\n' "$missing_required"
  exit 1
fi
printf '\nEverything required is installed.\n'
