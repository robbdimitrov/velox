#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8085}"
API_BASE="${API_BASE:-${BASE_URL}/api}"
RUN_ID="${RUN_ID:-$(date +%s)}"
TMP_DIR="${TMPDIR:-/tmp}/velox-smoke-${RUN_ID}"
BUYER_COOKIES="${TMP_DIR}/buyer.cookies"
ORGANIZER_COOKIES="${TMP_DIR}/organizer.cookies"

mkdir -p "$TMP_DIR"
touch "$BUYER_COOKIES" "$ORGANIZER_COOKIES"
trap 'rm -rf "$TMP_DIR"' EXIT

need() {
  command -v "$1" >/dev/null 2>&1 || {
    printf 'missing required tool: %s\n' "$1" >&2
    exit 1
  }
}

request() {
  local method="$1"
  local path="$2"
  local cookie_file="$3"
  local body="${4:-}"
  local out="${TMP_DIR}/response.json"
  local code

  if [[ -n "$body" ]]; then
    code="$(curl -sS -o "$out" -w '%{http_code}' -X "$method" \
      -H 'Content-Type: application/json' \
      -H "Origin: ${BASE_URL}" \
      -H "Idempotency-Key: smoke-${RUN_ID}-${method}-${path//\//-}" \
      -b "$cookie_file" -c "$cookie_file" \
      --data "$body" \
      "${API_BASE}${path}")"
  else
    code="$(curl -sS -o "$out" -w '%{http_code}' -X "$method" \
      -H "Origin: ${BASE_URL}" \
      -b "$cookie_file" -c "$cookie_file" \
      "${API_BASE}${path}")"
  fi

  if [[ "$code" -lt 200 || "$code" -ge 300 ]]; then
    printf '%s %s failed with HTTP %s\n' "$method" "$path" "$code" >&2
    cat "$out" >&2
    exit 1
  fi

  cat "$out"
}

need curl
need jq

printf 'checking gateway endpoints at %s\n' "$API_BASE"
request GET /healthz "$BUYER_COOKIES" >/dev/null
request GET /readyz "$BUYER_COOKIES" >/dev/null

buyer_email="buyer-${RUN_ID}@velox.local"
organizer_email="organizer-${RUN_ID}@velox.local"
password="velox-smoke-${RUN_ID}"

printf 'registering buyer %s\n' "$buyer_email"
request POST /auth/register "$BUYER_COOKIES" \
  "{\"email\":\"${buyer_email}\",\"password\":\"${password}\",\"role\":\"reserver\"}" >/dev/null

events_json="$(request GET /events "$BUYER_COOKIES")"
event_id="$(jq -r '.events[] | select(.seats_open > 1 and .status == "PUBLISHED") | .id' <<<"$events_json" | head -n 1)"
if [[ -z "$event_id" || "$event_id" == "null" ]]; then
  printf 'no published event with at least two open seats\n' >&2
  exit 1
fi

event_json="$(request GET "/events/${event_id}" "$BUYER_COOKIES")"
section_id="$(jq -r '.event.section_ids[0] // "A"' <<<"$event_json")"
seats_json="$(request GET "/events/${event_id}/sections/${section_id}/seats" "$BUYER_COOKIES")"
seat_ids="$(jq -r '[.seats[] | select(.status == "AVAILABLE") | .seat_id][0:2] | @json' <<<"$seats_json")"
if [[ "$seat_ids" == "[]" || "$seat_ids" == "null" ]]; then
  printf 'no available seats for %s section %s\n' "$event_id" "$section_id" >&2
  exit 1
fi

printf 'reserving seats for %s section %s\n' "$event_id" "$section_id"
reservation_json="$(request POST /reservations "$BUYER_COOKIES" \
  "{\"event_id\":\"${event_id}\",\"section_id\":\"${section_id}\",\"seat_ids\":${seat_ids}}")"
reservation_id="$(jq -r '.order.reservation_id' <<<"$reservation_json")"
order_id="$(jq -r '.order.id' <<<"$reservation_json")"

request GET "/orders/${order_id}" "$BUYER_COOKIES" >/dev/null
request POST "/reservations/${reservation_id}/confirm" "$BUYER_COOKIES" >/dev/null
request GET /wallet/tickets "$BUYER_COOKIES" >/dev/null

printf 'registering organizer %s\n' "$organizer_email"
request POST /auth/register "$ORGANIZER_COOKIES" \
  "{\"email\":\"${organizer_email}\",\"password\":\"${password}\",\"role\":\"organizer\"}" >/dev/null

venue_id="ven_smoke_${RUN_ID}"
request POST /organizer/venues "$ORGANIZER_COOKIES" \
  "{\"id\":\"${venue_id}\",\"name\":\"Smoke Hall ${RUN_ID}\",\"city\":\"Chicago\",\"address\":\"1 Smoke Way\",\"capacity\":10}" >/dev/null

event_new_id="evt_smoke_${RUN_ID}"
starts_at="$(date -u -v+30d '+%Y-%m-%dT%H:%M:%SZ' 2>/dev/null || date -u -d '+30 days' '+%Y-%m-%dT%H:%M:%SZ')"
sale_starts_at="$(date -u -v+1d '+%Y-%m-%dT%H:%M:%SZ' 2>/dev/null || date -u -d '+1 day' '+%Y-%m-%dT%H:%M:%SZ')"
request POST /organizer/events "$ORGANIZER_COOKIES" \
  "{\"id\":\"${event_new_id}\",\"venue_id\":\"${venue_id}\",\"name\":\"Smoke Event ${RUN_ID}\",\"description\":\"Smoke flow event created by scripts/smoke.sh.\",\"category\":\"Concerts\",\"starts_at\":\"${starts_at}\",\"sale_starts_at\":\"${sale_starts_at}\",\"image_key\":\"event-midnight-array\"}" >/dev/null

request POST "/organizer/events/${event_new_id}/announcements" "$ORGANIZER_COOKIES" \
  '{"title":"Doors update","body":"Doors open on schedule.","severity":"INFO"}' >/dev/null
request POST "/organizer/events/${event_new_id}/cancel" "$ORGANIZER_COOKIES" >/dev/null

printf 'smoke flow completed: buyer_order=%s organizer_event=%s\n' "$order_id" "$event_new_id"
