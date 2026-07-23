#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8085}"
API_BASE="${API_BASE:-${BASE_URL}/api}"
RUN_ID="${RUN_ID:-$(date +%s)}"
TMP_DIR="${TMPDIR:-/tmp}/velox-smoke-${RUN_ID}"
RESERVER_COOKIES="${TMP_DIR}/reserver.cookies"
ORGANIZER_COOKIES="${TMP_DIR}/organizer.cookies"
RESERVED_SEAT_COUNT=2
HOLD_POLL_ATTEMPTS=45
PROJECTION_POLL_ATTEMPTS=45

mkdir -p "$TMP_DIR"
touch "$RESERVER_COOKIES" "$ORGANIZER_COOKIES"
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

# Verifies a request fails with a specific stable error code, for negative-path
# checks; uses its own Idempotency-Key so it never collides with a prior
# request() call against the same path.
expect_request_error() {
  local method="$1"
  local path="$2"
  local cookie_file="$3"
  local body="$4"
  local idem_key="$5"
  local expected_error="$6"
  local out="${TMP_DIR}/response.json"
  local code

  code="$(curl -sS -o "$out" -w '%{http_code}' -X "$method" \
    -H 'Content-Type: application/json' \
    -H "Origin: ${BASE_URL}" \
    -H "Idempotency-Key: ${idem_key}" \
    -b "$cookie_file" -c "$cookie_file" \
    --data "$body" \
    "${API_BASE}${path}")"

  if [[ "$code" -lt 400 ]]; then
    printf '%s %s unexpectedly succeeded with HTTP %s; expected error %s\n' "$method" "$path" "$code" "$expected_error" >&2
    cat "$out" >&2
    exit 1
  fi
  if ! jq -e --arg err "$expected_error" '.error == $err' "$out" >/dev/null 2>&1; then
    printf '%s %s failed with HTTP %s but error was not %s\n' "$method" "$path" "$code" "$expected_error" >&2
    cat "$out" >&2
    exit 1
  fi
}

# Registers a user, or logs in instead if a prior run under the same RUN_ID
# already created it (register returns 409 user_already_exists).
register_or_login() {
  local cookie_file="$1"
  local email="$2"
  local role="$3"
  local out="${TMP_DIR}/register.json"
  local code

  code="$(curl -sS -o "$out" -w '%{http_code}' -X POST \
    -H 'Content-Type: application/json' \
    -H "Origin: ${BASE_URL}" \
    -b "$cookie_file" -c "$cookie_file" \
    --data "{\"email\":\"${email}\",\"password\":\"${password}\",\"role\":\"${role}\"}" \
    "${API_BASE}/auth/register")"

  if [[ "$code" -ge 200 && "$code" -lt 300 ]]; then
    return
  fi
  if [[ "$code" == "409" ]] && jq -e '.error == "user_already_exists"' "$out" >/dev/null 2>&1; then
    printf '%s already registered from a prior run; logging in\n' "$email"
    request POST /auth/login "$cookie_file" \
      "{\"email\":\"${email}\",\"password\":\"${password}\"}" >/dev/null
    return
  fi
  printf 'POST /auth/register failed with HTTP %s\n' "$code" >&2
  cat "$out" >&2
  exit 1
}

# Bounded poll: retries check_fn (which performs its own request and returns
# non-zero while not yet satisfied) once per second up to max_attempts.
wait_until() {
  local description="$1"
  local max_attempts="$2"
  local check_fn="$3"
  local attempt=0

  while [[ "$attempt" -lt "$max_attempts" ]]; do
    attempt=$((attempt + 1))
    if "$check_fn"; then
      return 0
    fi
    sleep 1
  done
  printf 'timed out waiting for %s\n' "$description" >&2
  return 1
}

need curl
need jq

printf 'checking gateway endpoints at %s\n' "$API_BASE"
request GET /healthz "$RESERVER_COOKIES" >/dev/null
request GET /readyz "$RESERVER_COOKIES" >/dev/null

reserver_email="reserver-${RUN_ID}@velox.local"
organizer_email="organizer-${RUN_ID}@velox.local"
password="velox-smoke-${RUN_ID}"

printf 'authenticating reserver %s\n' "$reserver_email"
register_or_login "$RESERVER_COOKIES" "$reserver_email" "reserver"

printf 'authenticating organizer %s\n' "$organizer_email"
register_or_login "$ORGANIZER_COOKIES" "$organizer_email" "organizer"

venue_id="ven_smoke_${RUN_ID}"
printf 'organizer creating venue %s\n' "$venue_id"
request POST /organizer/venues "$ORGANIZER_COOKIES" \
  "{\"id\":\"${venue_id}\",\"name\":\"Smoke Hall ${RUN_ID}\",\"city\":\"Chicago\",\"address\":\"1 Smoke Way\",\"capacity\":10,\"sections\":[{\"section_id\":\"A\",\"name\":\"Main Floor\",\"row_count\":2,\"seats_per_row\":5,\"accessible_edge_seats\":true}]}" >/dev/null

event_id="evt_smoke_${RUN_ID}"
starts_at="$(date -u -v+30d '+%Y-%m-%dT%H:%M:%SZ' 2>/dev/null || date -u -d '+30 days' '+%Y-%m-%dT%H:%M:%SZ')"
printf 'organizer creating event %s on venue %s\n' "$event_id" "$venue_id"
request POST /organizer/events "$ORGANIZER_COOKIES" \
  "{\"id\":\"${event_id}\",\"venue_id\":\"${venue_id}\",\"name\":\"Smoke Event ${RUN_ID}\",\"description\":\"Smoke flow event created by scripts/smoke.sh.\",\"category\":\"Concerts\",\"starts_at\":\"${starts_at}\"}" >/dev/null

printf 'buyer discovering event %s\n' "$event_id"
events_json="$(request GET '/events?available=false' "$RESERVER_COOKIES")"
found_event_count="$(jq -r --arg id "$event_id" '[.events[] | select(.id == $id)] | length' <<<"$events_json")"
if [[ "$found_event_count" != "1" ]]; then
  printf 'organizer-created event %s was not found via GET /events\n' "$event_id" >&2
  exit 1
fi

event_json="$(request GET "/events/${event_id}" "$RESERVER_COOKIES")"
section_id="$(jq -r '.event.section_ids[0] // "A"' <<<"$event_json")"
seats_json="$(request GET "/events/${event_id}/sections/${section_id}/seats" "$RESERVER_COOKIES")"
seat_ids="$(jq -r --argjson count "$RESERVED_SEAT_COUNT" '[.seats[] | select(.status == "AVAILABLE") | .seat_id][0:$count] | @json' <<<"$seats_json")"
if [[ "$(jq -r 'length' <<<"$seat_ids")" != "$RESERVED_SEAT_COUNT" ]]; then
  printf 'expected %s available seats for %s section %s\n' "$RESERVED_SEAT_COUNT" "$event_id" "$section_id" >&2
  exit 1
fi

printf 'reserving seats for %s section %s\n' "$event_id" "$section_id"
reservation_json="$(request POST /reservations "$RESERVER_COOKIES" \
  "{\"event_id\":\"${event_id}\",\"section_id\":\"${section_id}\",\"seat_ids\":${seat_ids}}")"
reservation_id="$(jq -r '.order.reservation_id' <<<"$reservation_json")"
reservation_token="$(jq -r '.order.reservation_token' <<<"$reservation_json")"
order_id="$(jq -r '.order.id' <<<"$reservation_json")"
if [[ -z "$reservation_token" || "$reservation_token" == "null" || "$reservation_token" == "$reservation_id" ]]; then
  printf 'reservation token missing or not signed\n' >&2
  exit 1
fi

order_status=""
check_order_held() {
  local order_json
  order_json="$(request GET "/orders/${order_id}" "$RESERVER_COOKIES")"
  order_status="$(jq -r '.order.status' <<<"$order_json")"
  [[ "$order_status" == "HELD" || "$order_status" == "CONFIRMED" ]]
}
printf 'waiting for hold on order %s\n' "$order_id"
if ! wait_until "order ${order_id} to reach HELD" "$HOLD_POLL_ATTEMPTS" check_order_held; then
  printf 'last status=%s\n' "$order_status" >&2
  exit 1
fi

confirm_out="${TMP_DIR}/confirm.json"
confirm_code="$(curl -sS -o "$confirm_out" -w '%{http_code}' -X POST \
  -H "Origin: ${BASE_URL}" \
  -H "Idempotency-Key: smoke-${RUN_ID}-confirm-${reservation_id}" \
  -H "Reservation-Token: ${reservation_token}" \
  -b "$RESERVER_COOKIES" -c "$RESERVER_COOKIES" \
  "${API_BASE}/reservations/${reservation_id}/confirm")"
if [[ "$confirm_code" -lt 200 || "$confirm_code" -ge 300 ]]; then
  printf 'POST /reservations/%s/confirm failed with HTTP %s\n' "$reservation_id" "$confirm_code" >&2
  cat "$confirm_out" >&2
  exit 1
fi
jq -e '.wallet_ticket_ids | type == "array"' "$confirm_out" >/dev/null
confirm_ticket_ids="$(jq -c '.wallet_ticket_ids // []' "$confirm_out")"

# wallet_ticket_ids may be empty in the confirm response while the wallet
# projection catches up, so the source of truth for ticket IDs is the wallet
# read once RESERVED_SEAT_COUNT tickets for this event appear there.
wallet_ticket_ids="[]"
check_wallet_tickets_issued() {
  local wallet_json ids count
  wallet_json="$(request GET /wallet/tickets "$RESERVER_COOKIES")"
  ids="$(jq -c --arg event_id "$event_id" '[.tickets[] | select(.event_id == $event_id) | .ticket_id]' <<<"$wallet_json")"
  count="$(jq -r 'length' <<<"$ids")"
  if [[ "$count" -ge "$RESERVED_SEAT_COUNT" ]]; then
    wallet_ticket_ids="$ids"
    return 0
  fi
  return 1
}
printf 'waiting for wallet ticket issuance for order %s\n' "$order_id"
wait_until "wallet tickets for event ${event_id}" "$PROJECTION_POLL_ATTEMPTS" check_wallet_tickets_issued || exit 1

if [[ "$confirm_ticket_ids" != "[]" ]]; then
  missing="$(jq -cn --argjson want "$confirm_ticket_ids" --argjson have "$wallet_ticket_ids" '$want - $have')"
  if [[ "$(jq -r 'length' <<<"$missing")" != "0" ]]; then
    printf 'wallet_ticket_ids from confirm not found in wallet: %s\n' "$missing" >&2
    exit 1
  fi
fi

printf 'organizer posting announcement on event %s\n' "$event_id"
request POST "/organizer/events/${event_id}/announcements" "$ORGANIZER_COOKIES" \
  '{"title":"Doors update","body":"Doors open on schedule.","severity":"INFO"}' >/dev/null

printf 'buyer fetching announcements for event %s\n' "$event_id"
announcements_json="$(request GET "/events/${event_id}/announcements" "$RESERVER_COOKIES")"
announcement_found="$(jq -r '[.announcements[] | select(.title == "Doors update")] | length' <<<"$announcements_json")"
if [[ "$announcement_found" == "0" ]]; then
  printf 'posted announcement not found in /events/%s/announcements\n' "$event_id" >&2
  exit 1
fi

printf 'organizer cancelling event %s\n' "$event_id"
request POST "/organizer/events/${event_id}/cancel" "$ORGANIZER_COOKIES" >/dev/null

printf 'verifying event %s is no longer bookable\n' "$event_id"
expect_request_error POST /reservations "$RESERVER_COOKIES" \
  "{\"event_id\":\"${event_id}\",\"section_id\":\"${section_id}\",\"seat_ids\":${seat_ids}}" \
  "smoke-${RUN_ID}-reservations-post-cancel-check" \
  "event_not_bookable"

sample_ticket_id="$(jq -r '.[0]' <<<"$wallet_ticket_ids")"
check_wallet_ticket_cancelled() {
  local wallet_json status
  wallet_json="$(request GET /wallet/tickets "$RESERVER_COOKIES")"
  status="$(jq -r --arg id "$sample_ticket_id" '[.tickets[] | select(.ticket_id == $id) | .status][0] // empty' <<<"$wallet_json")"
  [[ "$status" == "CANCELLED" ]]
}
printf 'waiting for wallet ticket %s to be cancelled\n' "$sample_ticket_id"
wait_until "wallet ticket ${sample_ticket_id} to become CANCELLED" "$PROJECTION_POLL_ATTEMPTS" check_wallet_ticket_cancelled || exit 1

printf 'smoke flow completed: event=%s order=%s wallet_ticket_ids=%s\n' "$event_id" "$order_id" "$wallet_ticket_ids"
