#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
DEPLOY_DIR="${ROOT_DIR}/deploy"
NS="${NS:-velox}"
KUBECTL="${KUBECTL:-kubectl}"
LOCAL_FRONTEND_PORT="${LOCAL_FRONTEND_PORT:-8080}"
LOCAL_GATEWAY_PORT="${LOCAL_GATEWAY_PORT:-8081}"
FRONTEND_PORT_FORWARD_LOG="${FRONTEND_PORT_FORWARD_LOG:-/tmp/velox-frontend-port-forward-${LOCAL_FRONTEND_PORT}.log}"
GATEWAY_PORT_FORWARD_LOG="${GATEWAY_PORT_FORWARD_LOG:-/tmp/velox-gateway-port-forward-${LOCAL_GATEWAY_PORT}.log}"
FRONTEND_PORT_FORWARD_PID_FILE="${FRONTEND_PORT_FORWARD_PID_FILE:-/tmp/velox-frontend-port-forward-${LOCAL_FRONTEND_PORT}.pid}"
GATEWAY_PORT_FORWARD_PID_FILE="${GATEWAY_PORT_FORWARD_PID_FILE:-/tmp/velox-gateway-port-forward-${LOCAL_GATEWAY_PORT}.pid}"
DRY_RUN="${DRY_RUN:-}"

ROLL_OUT_INFRA=(
  statefulset/postgres
  statefulset/redpanda
  deployment/dragonfly
)

ROLL_OUT_APPS=(
  deployment/apigateway
  deployment/orderservice
  deployment/inventoryservice
  deployment/projectionservice
  deployment/frontend
)

usage() {
  cat <<EOF
Usage: $0 [--dry-run] [--skip-build]

Environment:
  NS=${NS}
  LOCAL_FRONTEND_PORT=${LOCAL_FRONTEND_PORT}
  LOCAL_GATEWAY_PORT=${LOCAL_GATEWAY_PORT}
EOF
}

SKIP_BUILD="${SKIP_BUILD:-}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run)
      DRY_RUN=1
      ;;
    --skip-build)
      SKIP_BUILD=1
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      usage >&2
      exit 2
      ;;
  esac
  shift
done

log() {
  printf '==> %s\n' "$*"
}

die() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

random_secret() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex 32
    return
  fi
  LC_ALL=C tr -dc 'a-f0-9' </dev/urandom | head -c 64
}

require_tools() {
  $KUBECTL version --client >/dev/null 2>&1 || die "kubectl is not available through: ${KUBECTL}"
}

build_images() {
  if [[ -n "$SKIP_BUILD" || -n "$DRY_RUN" ]]; then
    return
  fi
  log "building Velox images"
  DOCKER_BUILDKIT=1 make -C "$ROOT_DIR"
}

apply_file() {
  local file="$1"
  if [[ -n "$DRY_RUN" ]]; then
    $KUBECTL apply --dry-run=client -f "$file"
  else
    $KUBECTL apply -f "$file"
  fi
}

ensure_namespace() {
  $KUBECTL create namespace "$NS" >/dev/null 2>&1 || true
}

ensure_secret() {
  local name="$1"
  shift
  if $KUBECTL -n "$NS" get secret "$name" >/dev/null 2>&1; then
    return
  fi
  $KUBECTL -n "$NS" create secret generic "$name" "$@"
}

ensure_dev_secrets() {
  if [[ -n "$DRY_RUN" ]]; then
    return
  fi
  log "ensuring generated development secrets"
  ensure_secret velox-postgres-secret --from-literal="password=$(random_secret)"
  ensure_secret velox-auth-secret \
    --from-literal="issuer=velox-dev" \
    --from-literal="audience=velox-browser" \
    --from-literal="session-secret=$(random_secret)"
  ensure_secret velox-kafka-signing-secret --from-literal="key=$(random_secret)"
}

apply_manifests() {
  log "applying manifests"
  apply_file "$DEPLOY_DIR/namespace.yaml"
  if [[ -z "$DRY_RUN" ]]; then
    ensure_namespace
  fi
  ensure_dev_secrets
  apply_file "$DEPLOY_DIR/postgres.yaml"
  apply_file "$DEPLOY_DIR/redpanda.yaml"
  apply_file "$DEPLOY_DIR/dragonfly.yaml"
  apply_file "$DEPLOY_DIR/services.yaml"
}

wait_for_rollouts() {
  if [[ -n "$DRY_RUN" ]]; then
    return
  fi
  local resource
  for resource in "$@"; do
    $KUBECTL -n "$NS" rollout status "$resource" --timeout=180s
  done
}

start_port_forward() {
  local service="$1"
  local local_port="$2"
  local remote_port="$3"
  local log_file="$4"
  local pid_file="$5"

  if [[ -n "$DRY_RUN" ]]; then
    return
  fi

  if [[ -s "$pid_file" ]] && ps -p "$(cat "$pid_file")" >/dev/null 2>&1; then
    log "port-forward for service/${service} already running on ${local_port}"
    return
  fi

  log "starting port-forward service/${service} ${local_port}:${remote_port}"
  nohup bash -c '
    set -u
    while true; do
      kubectl -n "$1" port-forward "service/$2" "$3:$4"
      sleep 3
    done
  ' bash "$NS" "$service" "$local_port" "$remote_port" >>"$log_file" 2>&1 &
  echo "$!" >"$pid_file"
}

print_summary() {
  if [[ -n "$DRY_RUN" ]]; then
    return
  fi
  cat <<EOF

==> Velox local runtime is starting

  Frontend   http://localhost:${LOCAL_FRONTEND_PORT}
  Gateway    http://localhost:${LOCAL_GATEWAY_PORT}
  Namespace  ${NS}

  Frontend port-forward pid: $(cat "$FRONTEND_PORT_FORWARD_PID_FILE" 2>/dev/null || echo unknown)
  Gateway port-forward pid:  $(cat "$GATEWAY_PORT_FORWARD_PID_FILE" 2>/dev/null || echo unknown)

  Pods:      kubectl -n ${NS} get pods
  Logs:      kubectl -n ${NS} logs deployment/<service> --tail=100
EOF
}

require_tools
build_images
apply_manifests
wait_for_rollouts "${ROLL_OUT_INFRA[@]}"
wait_for_rollouts "${ROLL_OUT_APPS[@]}"
start_port_forward frontend "$LOCAL_FRONTEND_PORT" 80 "$FRONTEND_PORT_FORWARD_LOG" "$FRONTEND_PORT_FORWARD_PID_FILE"
start_port_forward apigateway "$LOCAL_GATEWAY_PORT" 80 "$GATEWAY_PORT_FORWARD_LOG" "$GATEWAY_PORT_FORWARD_PID_FILE"
print_summary
