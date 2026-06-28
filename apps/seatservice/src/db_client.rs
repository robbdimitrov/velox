use crate::domain::SeatState;
use crate::dtos::{OrderCreatedPayload, SeatDto, SeatReservedEvent};
use chrono::{DateTime, Utc};
use serde_json::json;
use sqlx::{PgPool, Row};
use uuid::Uuid;

#[derive(Clone)]
pub struct DbClient {
    pub pool: PgPool,
}

impl DbClient {
    pub fn new(pool: PgPool) -> Self {
        Self { pool }
    }

    pub async fn process_reservation(
        &self,
        order: &OrderCreatedPayload,
        now: DateTime<Utc>,
    ) -> Result<Vec<SeatReservedEvent>, String> {
        let mut tx = self
            .pool
            .begin()
            .await
            .map_err(|e| format!("Failed to begin tx: {}", e))?;

        let mut stream_keys = vec![];
        let mut stream_versions = vec![];
        let mut all_available = true;

        let expires_at = now + chrono::Duration::minutes(10);
        let expires_at_ms = expires_at.timestamp_millis();

        // Check availability
        for seat_id in &order.seat_ids {
            let stream_key = format!("seat:{}:{}:{}", order.event_id, order.section_id, seat_id);

            // Ensure stream exists and lock it
            let _ = sqlx::query(
                "INSERT INTO inventory.event_streams (stream_key, event_id, section_id, seat_id, current_version) VALUES ($1, $2, $3, $4, 0) ON CONFLICT (stream_key) DO NOTHING"
            )
            .bind(&stream_key)
            .bind(&order.event_id)
            .bind(&order.section_id)
            .bind(seat_id)
            .execute(&mut *tx)
            .await
            .map_err(|e| format!("Failed to insert stream: {}", e))?;

            let stream_record = sqlx::query(
                "SELECT current_version FROM inventory.event_streams WHERE stream_key = $1 FOR UPDATE"
            )
            .bind(&stream_key)
            .fetch_one(&mut *tx)
            .await;

            let current_version: i32 = match stream_record {
                Ok(r) => r.get("current_version"),
                Err(_) => {
                    all_available = false;
                    break;
                }
            };
            stream_keys.push(stream_key.clone());
            stream_versions.push(current_version);

            // Load events to reconstruct state
            let events = sqlx::query(
                "SELECT event_type, payload FROM inventory.events WHERE stream_key = $1 ORDER BY aggregate_version ASC"
            )
            .bind(&stream_key)
            .fetch_all(&mut *tx)
            .await
            .map_err(|e| format!("Failed to load events: {}", e))?;

            let mut state = SeatState::default();
            for row in events {
                let event_type: String = row.get("event_type");
                let payload: serde_json::Value = row.get("payload");

                match event_type.as_str() {
                    "SeatReserved" => {
                        let order_id = payload
                            .get("order_id")
                            .and_then(|v| v.as_str())
                            .unwrap_or("")
                            .to_string();
                        // Assume 10 mins expiry from the event payload or we should really store expires_at_ms
                        // We will use expires_at_ms if present, otherwise default to a passed time to simulate it
                        let exp_ms = payload
                            .get("expires_at_ms")
                            .and_then(|v| v.as_i64())
                            .unwrap_or(0);
                        state.apply_reserved(order_id, exp_ms);
                    }
                    "SeatSold" => {
                        let order_id = payload
                            .get("order_id")
                            .and_then(|v| v.as_str())
                            .unwrap_or("")
                            .to_string();
                        state.apply_sold(order_id);
                    }
                    // Handle expiry event if we add one, otherwise we evaluate passively
                    _ => {}
                }
            }

            // Passively expire
            state.expire_if_due(now.timestamp_millis());

            if state.can_reserve(current_version as u64).is_err() {
                all_available = false;
                break;
            }
        }

        if !all_available {
            let _ = tx.rollback().await;
            return Err("Seats not available".into());
        }

        // Write reservation and events
        let order_uuid = match Uuid::parse_str(&order.order_id) {
            Ok(u) => u,
            Err(_) => return Err("Invalid order_id UUID".into()),
        };

        let _ = sqlx::query(
            "INSERT INTO inventory.reservations (reservation_id, order_id, user_id, status, expires_at) VALUES ($1, $2, $3, 'HELD', $4) ON CONFLICT (reservation_id) DO NOTHING"
        )
        .bind(&order.reservation_id)
        .bind(order_uuid)
        .bind(&order.user_id)
        .bind(expires_at)
        .execute(&mut *tx)
        .await
        .map_err(|e| format!("Failed to insert reservation: {}", e))?;

        let mut reserved_events = Vec::new();

        for i in 0..order.seat_ids.len() {
            let seat_id = &order.seat_ids[i];
            let stream_key = &stream_keys[i];
            let version = stream_versions[i] + 1;
            let event_uuid = Uuid::new_v4();

            let payload = json!({
                "order_id": order.order_id,
                "reservation_id": order.reservation_id,
                "user_id": order.user_id,
                "seat_id": seat_id,
                "section_id": order.section_id,
                "event_id": order.event_id,
                "expires_at_ms": expires_at_ms,
            });

            let _ = sqlx::query(
                "INSERT INTO inventory.events (id, stream_key, aggregate_version, event_type, payload, metadata, correlation_id, signature, occurred_at) VALUES ($1, $2, $3, 'SeatReserved', $4, '{}', $5, '\\x00', $6)"
            )
            .bind(event_uuid)
            .bind(stream_key)
            .bind(version)
            .bind(&payload)
            .bind(&order.order_id)
            .bind(now)
            .execute(&mut *tx)
            .await
            .map_err(|e| format!("Failed to insert event: {}", e))?;

            let _ = sqlx::query(
                "UPDATE inventory.event_streams SET current_version = $1, updated_at = $2 WHERE stream_key = $3"
            )
            .bind(version)
            .bind(now)
            .bind(stream_key)
            .execute(&mut *tx)
            .await
            .map_err(|e| format!("Failed to update stream: {}", e))?;

            reserved_events.push(SeatReservedEvent {
                event_id: event_uuid.to_string(),
                aggregate_id: stream_key.clone(),
                aggregate_version: version as u64,
                event_type: "SeatReserved".into(),
                correlation_id: order.order_id.clone(),
                seat: SeatDto {
                    event_id: order.event_id.clone(),
                    section_id: order.section_id.clone(),
                    seat_id: seat_id.clone(),
                    status: "HELD".into(),
                    version: version as u64,
                    expires_at_ms,
                },
                occurred_at: now,
            });
        }

        tx.commit()
            .await
            .map_err(|e| format!("Commit failed: {}", e))?;
        Ok(reserved_events)
    }
}
