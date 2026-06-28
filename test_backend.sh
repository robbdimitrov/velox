#!/bin/bash
set -e

echo "Starting Port Forward..."
kubectl -n velox port-forward svc/apigateway 8081:80 > /dev/null 2>&1 &
PF_PID=$!
sleep 2

echo "1. Creating Session..."
curl -s -X POST http://localhost:8081/sessions -H "Content-Type: application/json" -d '{"email":"reserver@velox.local","password":"reserver"}' -c cookie_be.txt | jq

echo -e "\n\n2. Creating Reservation..."
curl -s -X POST http://localhost:8081/reservations \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: $(uuidgen)" \
  -b cookie_be.txt \
  -d '{"event_id":"evt_neon_riot","section_id":"A","seat_ids":["A-01","A-02"]}' > reserve_be.json
cat reserve_be.json | jq
RES_ID=$(cat reserve_be.json | jq -r '.order.reservation_id')

echo -e "\n\n3. Confirming Reservation..."
curl -s -X POST http://localhost:8081/reservations/$RES_ID/confirm \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: $(uuidgen)" \
  -b cookie_be.txt \
  -d '{}' | jq

echo -e "\n\n4. Checking Order State..."
ORDER_ID=$(cat reserve_be.json | jq -r '.order.id')
sleep 2
curl -s http://localhost:8081/orders/$ORDER_ID -b cookie_be.txt | jq

kill $PF_PID
