#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
DEPLOY_DIR="${ROOT_DIR}/deploy"
NS="${NS:-velox}"
KUBECTL="${KUBECTL:-kubectl}"
IMAGE_REGISTRY="${IMAGE_REGISTRY:-ghcr.io/example/velox}"
IMAGE_TAG="${IMAGE_TAG:-dev}"
LOCAL_FRONTEND_PORT="${LOCAL_FRONTEND_PORT:-8085}"
FRONTEND_PORT_FORWARD_LOG="${FRONTEND_PORT_FORWARD_LOG:-/tmp/velox-frontend-port-forward-${LOCAL_FRONTEND_PORT}.log}"
FRONTEND_PORT_FORWARD_PID_FILE="${FRONTEND_PORT_FORWARD_PID_FILE:-/tmp/velox-frontend-port-forward-${LOCAL_FRONTEND_PORT}.pid}"
DRY_RUN="${DRY_RUN:-}"

ROLL_OUT_INFRA=(
  statefulset/database
  statefulset/broker
  deployment/cache
)

ROLL_OUT_APPS=(
  deployment/apigateway
  deployment/orderservice
  deployment/seatservice
  deployment/viewservice
  deployment/frontend
)

usage() {
  cat <<EOF
Usage: $0 [--dry-run] [--skip-build]

Environment:
  NS=${NS}
  IMAGE_REGISTRY=${IMAGE_REGISTRY}
  IMAGE_TAG=${IMAGE_TAG}
  LOCAL_FRONTEND_PORT=${LOCAL_FRONTEND_PORT}
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
  od -An -N32 -tx1 /dev/urandom | tr -d ' \n'
}

require_tools() {
  $KUBECTL version --client >/dev/null 2>&1 || die "kubectl is not available through: ${KUBECTL}"
}

build_images() {
  if [[ -n "$SKIP_BUILD" || -n "$DRY_RUN" ]]; then
    return
  fi
  log "building Velox images"
  DOCKER_BUILDKIT=1 IMAGE_REGISTRY="$IMAGE_REGISTRY" IMAGE_TAG="$IMAGE_TAG" make -C "$ROOT_DIR"
}

apply_file() {
  local file="$1"
  if [[ -n "$DRY_RUN" ]]; then
    $KUBECTL apply --dry-run=client -f "$file"
  else
    $KUBECTL apply -f "$file"
  fi
}

render_services_manifest() {
  local rendered
  rendered="$(mktemp)"
  sed \
    -e "s#ghcr.io/example/velox-apigateway:dev#${IMAGE_REGISTRY}-apigateway:${IMAGE_TAG}#g" \
    -e "s#ghcr.io/example/velox-orderservice:dev#${IMAGE_REGISTRY}-orderservice:${IMAGE_TAG}#g" \
    -e "s#ghcr.io/example/velox-seatservice:dev#${IMAGE_REGISTRY}-seatservice:${IMAGE_TAG}#g" \
    -e "s#ghcr.io/example/velox-viewservice:dev#${IMAGE_REGISTRY}-viewservice:${IMAGE_TAG}#g" \
    -e "s#ghcr.io/example/velox-frontend:dev#${IMAGE_REGISTRY}-frontend:${IMAGE_TAG}#g" \
    "$DEPLOY_DIR/services.yaml" >"$rendered"
  printf '%s\n' "$rendered"
}

ensure_namespace() {
  $KUBECTL create namespace "$NS" >/dev/null 2>&1 || true
}

ensure_secret() {
  local name="$1"
  shift
  if $KUBECTL -n "$NS" get secret "$name" >/dev/null 2>&1; then
    log "secret $name already exists, skipping"
    return
  fi
  $KUBECTL -n "$NS" create secret generic "$name" "$@" --dry-run=client -o yaml | $KUBECTL apply -f -
}

ensure_dev_secrets() {
  if [[ -n "$DRY_RUN" ]]; then
    return
  fi
  log "ensuring generated development secrets"
  ensure_secret velox-database-secret --from-literal="password=$(random_secret)"
  ensure_secret velox-auth-secret \
    --from-literal="issuer=velox-dev" \
    --from-literal="audience=velox-browser" \
    --from-literal="session-secret=$(random_secret)"
  ensure_secret velox-kafka-signing-secret --from-literal="key=$(random_secret)"
}

apply_manifests() {
  log "applying manifests"
  apply_file "$DEPLOY_DIR/namespace.yaml"
  apply_file "$DEPLOY_DIR/networkpolicy.yaml"
  if [[ -z "$DRY_RUN" ]]; then
    ensure_namespace
  fi
  ensure_dev_secrets
  apply_file "$DEPLOY_DIR/database.yaml"
  apply_file "$DEPLOY_DIR/broker.yaml"
  apply_file "$DEPLOY_DIR/cache.yaml"
  local services_manifest
  services_manifest="$(render_services_manifest)"
  trap 'rm -f "$services_manifest"' RETURN
  apply_file "$services_manifest"
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

  if port_forward_running "$pid_file" "$service" "$local_port" "$remote_port"; then
    log "port-forward for service/${service} already running on ${local_port}"
    return
  fi

  log "starting port-forward service/${service} ${local_port}:${remote_port}"
  nohup bash -c '
    set -u
    kubectl_cmd="$1"
    while true; do
      "$kubectl_cmd" -n "$2" port-forward "service/$3" "$4:$5"
      sleep 3
    done
  ' bash "$KUBECTL" "$NS" "$service" "$local_port" "$remote_port" >>"$log_file" 2>&1 &
  echo "$!" >"$pid_file"
}

port_forward_running() {
  local pid_file="$1"
  local service="$2"
  local local_port="$3"
  local remote_port="$4"
  if [[ ! -s "$pid_file" ]]; then
    return 1
  fi
  local pid
  pid="$(cat "$pid_file")"
  local command
  command="$(ps -p "$pid" -o command= 2>/dev/null || true)"
  [[ "$command" == *"port-forward service/${service} ${local_port}:${remote_port}"* || "$command" == *" ${service} ${local_port} ${remote_port}"* ]]
}

print_summary() {
  if [[ -n "$DRY_RUN" ]]; then
    return
  fi
  cat <<EOF

==> Velox local runtime is starting

  Frontend   http://localhost:${LOCAL_FRONTEND_PORT}
  Namespace  ${NS}

  Frontend port-forward pid: $(cat "$FRONTEND_PORT_FORWARD_PID_FILE" 2>/dev/null || echo unknown)

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
print_summary
