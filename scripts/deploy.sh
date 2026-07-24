#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
DEPLOY_DIR="${ROOT_DIR}/deploy"
NS="${NS:-velox}"
KUBECTL="${KUBECTL:-kubectl}"
IMAGE_PREFIX="${IMAGE_PREFIX:-localhost:5000/velox}"
IMAGE_DELIVERY="${IMAGE_DELIVERY:-auto}"
GIT_SHA="${GIT_SHA:-}"
APIGATEWAY_IMAGE_TAG="${APIGATEWAY_IMAGE_TAG:-}"
ORDERSERVICE_IMAGE_TAG="${ORDERSERVICE_IMAGE_TAG:-}"
SEATSERVICE_IMAGE_TAG="${SEATSERVICE_IMAGE_TAG:-}"
VIEWSERVICE_IMAGE_TAG="${VIEWSERVICE_IMAGE_TAG:-}"
FRONTEND_IMAGE_TAG="${FRONTEND_IMAGE_TAG:-}"
DATABASE_IMAGE_TAG="${DATABASE_IMAGE_TAG:-}"
LOCAL_FRONTEND_PORT="${LOCAL_FRONTEND_PORT:-8080}"
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
  IMAGE_PREFIX=${IMAGE_PREFIX}
  IMAGE_DELIVERY=${IMAGE_DELIVERY}
  GIT_SHA=${GIT_SHA:-<optional all-image tag override>}
  APIGATEWAY_IMAGE_TAG=${APIGATEWAY_IMAGE_TAG:-<content checksum>}
  ORDERSERVICE_IMAGE_TAG=${ORDERSERVICE_IMAGE_TAG:-<content checksum>}
  SEATSERVICE_IMAGE_TAG=${SEATSERVICE_IMAGE_TAG:-<content checksum>}
  VIEWSERVICE_IMAGE_TAG=${VIEWSERVICE_IMAGE_TAG:-<content checksum>}
  FRONTEND_IMAGE_TAG=${FRONTEND_IMAGE_TAG:-<content checksum>}
  DATABASE_IMAGE_TAG=${DATABASE_IMAGE_TAG:-<content checksum>}
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
  command -v openssl >/dev/null || die "missing required tool: openssl"
  command -v curl >/dev/null || die "missing required tool: curl"
  if [[ -n "$SKIP_BUILD" || -n "$DRY_RUN" ]]; then
    return
  fi
  local tool
  for tool in docker make; do
    command -v "$tool" >/dev/null || die "missing required tool: $tool"
  done
}

load_images_into_kind() {
  local node="velox-control-plane"
  local image
  docker container inspect --format '{{.State.Running}}' "$node" 2>/dev/null | grep -qx true || return 0
  log "loading images into kind node"
  for image in \
    "${IMAGE_PREFIX}-apigateway:${APIGATEWAY_IMAGE_TAG}" \
    "${IMAGE_PREFIX}-orderservice:${ORDERSERVICE_IMAGE_TAG}" \
    "${IMAGE_PREFIX}-seatservice:${SEATSERVICE_IMAGE_TAG}" \
    "${IMAGE_PREFIX}-viewservice:${VIEWSERVICE_IMAGE_TAG}" \
    "${IMAGE_PREFIX}-frontend:${FRONTEND_IMAGE_TAG}" \
    "${IMAGE_PREFIX}-database:${DATABASE_IMAGE_TAG}"; do
    docker save "$image" | docker exec -i "$node" ctr --namespace k8s.io images import -
  done
}

local_cluster_images_visible() {
  local kube_context docker_context
  kube_context="$($KUBECTL config current-context 2>/dev/null || true)"
  docker_context="$(docker context show 2>/dev/null || true)"
  if [[ "$kube_context" == colima && "$docker_context" == colima ]]; then
    return 0
  fi
  if [[ "$kube_context" == kind-velox ]]; then
    docker container inspect velox-control-plane >/dev/null 2>&1
    return
  fi
  return 1
}

resolve_image_delivery() {
  case "$IMAGE_DELIVERY" in
    auto)
      if local_cluster_images_visible; then
        printf 'local\n'
      else
        printf 'push\n'
      fi
      ;;
    local|push)
      printf '%s\n' "$IMAGE_DELIVERY"
      ;;
    *)
      die "IMAGE_DELIVERY must be auto, local, or push"
      ;;
  esac
}

context_checksum() {
  local dir="$1"
  (
    cd "${ROOT_DIR}/${dir}"
    if [[ -f "${ROOT_DIR}/Makefile" ]]; then
      printf '%s\0' "../../Makefile"
      openssl dgst -sha256 -binary "${ROOT_DIR}/Makefile"
    fi
    find . -type f \
      ! -path './.git/*' \
      ! -path './bin/*' \
      ! -path './tmp/*' \
      ! -path './coverage/*' \
      ! -path './node_modules/*' \
      ! -path './target/*' \
      ! -path './.svelte-kit/*' \
      ! -path './build/*' \
      ! -path './dist/*' \
      -print |
      LC_ALL=C sort |
      while IFS= read -r file; do
        case "${dir}:${file}" in
          apps/apigateway:./apigateway | apps/orderservice:./orderservice | apps/viewservice:./viewservice)
            continue
            ;;
          apps/frontend:*.md | apps/frontend:./.env | apps/frontend:./.env.*)
            [[ "$file" == "./.env.example" ]] || continue
            ;;
        esac
        printf '%s\0' "$file"
        openssl dgst -sha256 -binary "$file"
      done
  ) | openssl dgst -sha256 -r | awk '{print substr($1, 1, 12)}'
}

init_image_tags() {
  APIGATEWAY_IMAGE_TAG="${APIGATEWAY_IMAGE_TAG:-${GIT_SHA:-$(context_checksum apps/apigateway)}}"
  ORDERSERVICE_IMAGE_TAG="${ORDERSERVICE_IMAGE_TAG:-${GIT_SHA:-$(context_checksum apps/orderservice)}}"
  SEATSERVICE_IMAGE_TAG="${SEATSERVICE_IMAGE_TAG:-${GIT_SHA:-$(context_checksum apps/seatservice)}}"
  VIEWSERVICE_IMAGE_TAG="${VIEWSERVICE_IMAGE_TAG:-${GIT_SHA:-$(context_checksum apps/viewservice)}}"
  FRONTEND_IMAGE_TAG="${FRONTEND_IMAGE_TAG:-${GIT_SHA:-$(context_checksum apps/frontend)}}"
  DATABASE_IMAGE_TAG="${DATABASE_IMAGE_TAG:-${GIT_SHA:-$(context_checksum apps/database)}}"
}

build_images() {
  if [[ -n "$SKIP_BUILD" || -n "$DRY_RUN" ]]; then
    return
  fi
  local image_delivery push_images
  image_delivery="$(resolve_image_delivery)"
  push_images=1
  if [[ "$image_delivery" == "local" ]]; then
    push_images=0
  fi
  log "building Velox images"
  log "image delivery: ${image_delivery}"
  log "image tags: apigateway=${APIGATEWAY_IMAGE_TAG} orderservice=${ORDERSERVICE_IMAGE_TAG} seatservice=${SEATSERVICE_IMAGE_TAG} viewservice=${VIEWSERVICE_IMAGE_TAG} frontend=${FRONTEND_IMAGE_TAG} database=${DATABASE_IMAGE_TAG}"
  DOCKER_BUILDKIT=1 PUSH_IMAGES="$push_images" IMAGE_PREFIX="$IMAGE_PREFIX" GIT_SHA="$APIGATEWAY_IMAGE_TAG" make -C "$ROOT_DIR" apigateway
  DOCKER_BUILDKIT=1 PUSH_IMAGES="$push_images" IMAGE_PREFIX="$IMAGE_PREFIX" GIT_SHA="$ORDERSERVICE_IMAGE_TAG" make -C "$ROOT_DIR" orderservice
  DOCKER_BUILDKIT=1 PUSH_IMAGES="$push_images" IMAGE_PREFIX="$IMAGE_PREFIX" GIT_SHA="$SEATSERVICE_IMAGE_TAG" make -C "$ROOT_DIR" seatservice
  DOCKER_BUILDKIT=1 PUSH_IMAGES="$push_images" IMAGE_PREFIX="$IMAGE_PREFIX" GIT_SHA="$VIEWSERVICE_IMAGE_TAG" make -C "$ROOT_DIR" viewservice
  DOCKER_BUILDKIT=1 PUSH_IMAGES="$push_images" IMAGE_PREFIX="$IMAGE_PREFIX" GIT_SHA="$FRONTEND_IMAGE_TAG" make -C "$ROOT_DIR" frontend
  DOCKER_BUILDKIT=1 PUSH_IMAGES="$push_images" IMAGE_PREFIX="$IMAGE_PREFIX" GIT_SHA="$DATABASE_IMAGE_TAG" make -C "$ROOT_DIR" database
  load_images_into_kind
}

apply_file() {
  local file="$1"
  if [[ -n "$DRY_RUN" ]]; then
    $KUBECTL apply --dry-run=client -n "$NS" -f "$file"
  else
    $KUBECTL apply -n "$NS" -f "$file"
  fi
}

render_manifest() {
  local file="$1"
  local rendered
  rendered="$(mktemp)"
  sed \
    -e "s#ghcr.io/example/velox-apigateway:dev#${IMAGE_PREFIX}-apigateway:${APIGATEWAY_IMAGE_TAG}#g" \
    -e "s#ghcr.io/example/velox-orderservice:dev#${IMAGE_PREFIX}-orderservice:${ORDERSERVICE_IMAGE_TAG}#g" \
    -e "s#ghcr.io/example/velox-seatservice:dev#${IMAGE_PREFIX}-seatservice:${SEATSERVICE_IMAGE_TAG}#g" \
    -e "s#ghcr.io/example/velox-viewservice:dev#${IMAGE_PREFIX}-viewservice:${VIEWSERVICE_IMAGE_TAG}#g" \
    -e "s#ghcr.io/example/velox-frontend:dev#${IMAGE_PREFIX}-frontend:${FRONTEND_IMAGE_TAG}#g" \
    -e "s#ghcr.io/example/velox-database:dev#${IMAGE_PREFIX}-database:${DATABASE_IMAGE_TAG}#g" \
    "$file" >"$rendered"
  printf '%s\n' "$rendered"
}

ensure_namespace() {
  if [[ -n "$DRY_RUN" ]]; then
    return
  fi
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
  ensure_secret velox-event-signing-secret --from-literal="key=$(random_secret)"
  ensure_secret velox-order-event-signing-secret --from-literal="key=$(random_secret)"
}

apply_manifests() {
  log "applying base manifests"
  ensure_namespace
  apply_file "$DEPLOY_DIR/serviceaccounts.yaml"
  apply_file "$DEPLOY_DIR/networkpolicy.yaml"
  ensure_dev_secrets
}

data_resource_checksum() {
  local kind="$1"
  local name="$2"
  $KUBECTL -n "$NS" get "$kind" "$name" -o go-template='{{ range $k, $v := .data }}{{ printf "%s=%s\n" $k $v }}{{ end }}' \
    | LC_ALL=C sort \
    | openssl dgst -sha256 -r | awk '{print $1}'
}

annotate_data_resource_checksums() {
  if [[ -n "$DRY_RUN" ]]; then
    return
  fi
  local kind="$1"
  local resource="$2"
  shift 2
  local name pairs=()
  for name in "$@"; do
    pairs+=("\"checksum/${name}\":\"$(data_resource_checksum "$kind" "$name")\"")
  done
  local joined
  joined="$(IFS=,; echo "${pairs[*]}")"
  $KUBECTL -n "$NS" patch "$resource" --type merge \
    -p "{\"spec\":{\"template\":{\"metadata\":{\"annotations\":{${joined}}}}}}" >/dev/null
}

annotate_secret_checksums() {
  local resource="$1"
  shift
  annotate_data_resource_checksums secret "$resource" "$@"
}

annotate_configmap_checksums() {
  local resource="$1"
  shift
  annotate_data_resource_checksums configmap "$resource" "$@"
}

apply_infra_manifests() {
  log "applying infrastructure manifests"
  local database_manifest
  database_manifest="$(render_manifest "$DEPLOY_DIR/database.yaml")"
  trap 'rm -f "$database_manifest"' RETURN
  apply_file "$database_manifest"
  apply_file "$DEPLOY_DIR/broker.yaml"
  apply_file "$DEPLOY_DIR/cache.yaml"
  annotate_secret_checksums statefulset/database velox-database-secret
  annotate_configmap_checksums statefulset/database velox-database-config
  trap - RETURN
  rm -f "$database_manifest"
}

apply_app_manifests() {
  log "applying application manifests"
  local services_manifest
  services_manifest="$(render_manifest "$DEPLOY_DIR/services.yaml")"
  trap 'rm -f "$services_manifest"' RETURN
  apply_file "$services_manifest"
  apply_file "$DEPLOY_DIR/pdb.yaml"
  annotate_configmap_checksums deployment/apigateway velox-service-config
  annotate_secret_checksums deployment/apigateway velox-database-secret velox-auth-secret
  annotate_configmap_checksums deployment/orderservice velox-service-config
  annotate_secret_checksums deployment/orderservice velox-database-secret velox-kafka-signing-secret velox-order-event-signing-secret velox-event-signing-secret
  annotate_configmap_checksums deployment/seatservice velox-service-config
  annotate_secret_checksums deployment/seatservice velox-database-secret velox-kafka-signing-secret velox-event-signing-secret velox-order-event-signing-secret
  annotate_configmap_checksums deployment/viewservice velox-service-config
  annotate_secret_checksums deployment/viewservice velox-database-secret velox-kafka-signing-secret velox-event-signing-secret
  annotate_configmap_checksums deployment/frontend velox-service-config
  trap - RETURN
  rm -f "$services_manifest"
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

# Ensures Kafka topics exist before app services start. Auto-create-on-produce
# is unreliable on cold brokers, and Jobs are recreated for idempotent redeploys.
apply_topics_job() {
  if [[ -n "$DRY_RUN" ]]; then
    apply_file "$DEPLOY_DIR/topics.yaml"
    return
  fi
  log "provisioning Kafka topics"
  $KUBECTL -n "$NS" delete job broker-topics-init --ignore-not-found >/dev/null
  apply_file "$DEPLOY_DIR/topics.yaml"
  $KUBECTL -n "$NS" wait --for=condition=complete job/broker-topics-init --timeout=60s
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

  if [[ "$service" == "frontend" ]] && frontend_port_forward_ready "$local_port"; then
    log "frontend already reachable on ${local_port}"
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

frontend_port_forward_ready() {
  local local_port="$1"
  curl -fsS --max-time 2 "http://127.0.0.1:${local_port}/api/healthz" >/dev/null 2>&1
}

wait_for_frontend_port_forward() {
  local local_port="$1"
  local attempt
  if [[ -n "$DRY_RUN" ]]; then
    return
  fi
  for attempt in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15; do
    if frontend_port_forward_ready "$local_port"; then
      return
    fi
    sleep 1
  done
  die "frontend port-forward did not become ready on ${local_port}"
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

  Frontend   http://velox.localhost:${LOCAL_FRONTEND_PORT}
  Namespace  ${NS}

  Frontend port-forward pid: $(cat "$FRONTEND_PORT_FORWARD_PID_FILE" 2>/dev/null || echo unknown)

  Pods:      kubectl -n ${NS} get pods
  Logs:      kubectl -n ${NS} logs deployment/<service> --tail=100
EOF
}

require_tools
init_image_tags
build_images
apply_manifests
apply_infra_manifests
wait_for_rollouts "${ROLL_OUT_INFRA[@]}"
apply_topics_job
apply_app_manifests
wait_for_rollouts "${ROLL_OUT_APPS[@]}"
start_port_forward frontend "$LOCAL_FRONTEND_PORT" 80 "$FRONTEND_PORT_FORWARD_LOG" "$FRONTEND_PORT_FORWARD_PID_FILE"
wait_for_frontend_port_forward "$LOCAL_FRONTEND_PORT"
print_summary
