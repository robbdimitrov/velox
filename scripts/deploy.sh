#!/usr/bin/env sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
K8S_DIR="${ROOT_DIR}/deploy"
KUBECTL="${KUBECTL:-rtk kubectl}"
DRY_RUN="${DRY_RUN:-}"

usage() {
  printf 'Usage: %s [--dry-run]\n' "$0"
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --dry-run)
      DRY_RUN=1
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      usage
      exit 2
      ;;
  esac
  shift
done

if ! command -v rtk >/dev/null 2>&1; then
  printf 'rtk is required on PATH\n' >&2
  exit 127
fi

apply_file() {
  file="$1"
  if [ -n "$DRY_RUN" ]; then
    $KUBECTL apply --dry-run=client -f "$file"
  else
    $KUBECTL apply -f "$file"
  fi
}

printf 'Applying Velox manifests to namespace velox\n'
apply_file "$K8S_DIR/namespace.yaml"
apply_file "$K8S_DIR/postgres.yaml"
apply_file "$K8S_DIR/redpanda.yaml"
apply_file "$K8S_DIR/dragonfly.yaml"
apply_file "$K8S_DIR/services.yaml"

if [ -z "$DRY_RUN" ]; then
  printf '\nRequired secrets are referenced but not created by this repository:\n'
  printf '  kubectl -n velox create secret generic velox-postgres-secret --from-literal=password=...\n'
  printf '  kubectl -n velox create secret generic velox-auth-secret --from-literal=issuer=... --from-literal=audience=...\n'
  printf '  kubectl -n velox create secret generic velox-kafka-signing-secret --from-literal=key=...\n'
fi
