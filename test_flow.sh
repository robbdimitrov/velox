#!/bin/bash
set -e

echo "1. Logging in..."
curl -s -X POST http://localhost:8085/api/auth/login -H "Content-Type: application/json" -H "Origin: http://localhost:8085" -d '{"email":"reserver@velox.local"}' -c cookies.txt

echo -e "\n\n2. Fetching Events..."
curl -s http://localhost:8085/api/events -H "Origin: http://localhost:8085" -b cookies.txt | jq

echo -e "\n\n3. Fetching Seats for evt_neon_riot Section A..."
curl -s http://localhost:8085/api/events/evt_neon_riot/sections/A/seats -H "Origin: http://localhost:8085" -b cookies.txt | jq '.seats[0:2]'

echo -e "\n\n4. Holding Seats..."
curl -s -X POST http://localhost:8085/api/events/evt_neon_riot/sections/A/reservations \
  -H "Content-Type: application/json" \
  -H "Origin: http://localhost:8085" \
  -H "Idempotency-Key: $(uuidgen)" \
  -b cookies.txt \
  -d '{"event_id":"evt_neon_riot","section_id":"A","seat_ids":["A-01","A-02"]}' > reserve.json
cat reserve.json | jq
ORDER_ID=$(cat reserve.json | jq -r '.order.id')

echo -e "\n\n5. Checking Order State..."
curl -s http://localhost:8085/api/orders/$ORDER_ID -H "Origin: http://localhost:8085" -b cookies.txt | jq

echo -e "\n\n6. Confirming Reservation..."
curl -s -X POST http://localhost:8085/api/orders/$ORDER_ID/confirm \
  -H "Content-Type: application/json" \
  -H "Origin: http://localhost:8085" \
  -H "Idempotency-Key: $(uuidgen)" \
  -b cookies.txt \
  -d '{"reservation_id":"res_'$ORDER_ID'"}' | jq

echo -e "\n\n7. Wait a moment and check final Order State..."
sleep 2
curl -s http://localhost:8085/api/orders/$ORDER_ID -H "Origin: http://localhost:8085" -b cookies.txt | jq

