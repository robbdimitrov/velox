#!/usr/bin/env sh
set -eu

namespace="${NAMESPACE:-velox}"
timeout="${TIMEOUT:-180s}"

restart() {
  kind="$1"
  name="$2"
  if kubectl -n "$namespace" get "$kind" "$name" >/dev/null 2>&1; then
    kubectl -n "$namespace" rollout restart "$kind/$name"
    kubectl -n "$namespace" rollout status "$kind/$name" --timeout="$timeout"
  fi
}

check_deployment() {
  name="$1"
  kubectl -n "$namespace" wait --for=condition=available "deployment/$name" --timeout="$timeout"
}

restart statefulset database
restart deployment cache
restart statefulset broker

check_deployment apigateway
check_deployment orderservice
check_deployment seatservice
check_deployment viewservice

echo "dependency restart drill completed for namespace $namespace"
