use crate::domain::SeatState;
use crate::dtos::{
    OrderCreatedPayload, SeatDto, SeatInventoryEvent, SeatReservationFailedEvent,
    INVENTORY_EVENT_SCHEMA_VERSION,
};
use crate::signing;
use chrono::{DateTime, Utc};
use serde_json::json;
use sqlx::{PgPool, Postgres, Row, Transaction};
use uuid::Uuid;

const EXPIRY_SCHEDULER_CAUSATION_ID: &str = "seatservice:expiry-scheduler";
const MAX_EXPIRING_RESERVATIONS_PER_SWEEP: i64 = 100;

#[derive(Clone)]
pub struct DbClient {
    pub pool: PgPool,
    signing_key: Vec<u8>,
}

struct FoldedStream {
    stream_key: String,
    current_version: i32,
    state: SeatState,
}

/// The event_id/section_id/seat_id fields carried on every SeatReservationHeld
/// payload, re-extracted whenever a later event (expiry, confirmation) needs
/// to know which seat a stream refers to.
struct HeldSeatFields {
    event_id: String,
    section_id: String,
    seat_id: String,
}

/// Parameters for appending one event to a stream. Grouped into a struct
/// rather than passed as separate arguments to append_event.
struct EventAppend<'a> {
    stream_key: &'a str,
    version: i32,
    event_type: &'a str,
    payload: &'a serde_json::Value,
    correlation_id: &'a str,
    causation_id: &'a str,
}

impl DbClient {
    pub fn new(pool: PgPool) -> Self {
        Self {
            pool,
            signing_key: signing::signing_key(),
        }
    }

    /// Atomically claims an inbound event_id for processing. Returns `false` if the
    /// event was already processed (drop as a duplicate per the idempotent-consumer rule).
    async fn claim_event(
        tx: &mut Transaction<'_, Postgres>,
        event_id: &str,
        event_type: &str,
    ) -> Result<bool, String> {
        let result = sqlx::query(
            "INSERT INTO inventory.processed_events (event_id, event_type) VALUES ($1, $2) ON CONFLICT (event_id) DO NOTHING"
        )
        .bind(event_id)
        .bind(event_type)
        .execute(&mut **tx)
        .await
        .map_err(|e| format!("Failed to claim event: {}", e))?;
        Ok(result.rows_affected() > 0)
    }

    fn sign(
        &self,
        event_type: &str,
        aggregate_id: &str,
        aggregate_version: u64,
        payload: &serde_json::Value,
    ) -> String {
        let payload_bytes = serde_json::to_vec(payload).unwrap_or_default();
        let sig = signing::sign(
            &self.signing_key,
            event_type,
            aggregate_id,
            aggregate_version,
            &payload_bytes,
        );
        hex::encode(sig)
    }

    /// Appends one row to inventory.events and bumps the owning stream's
    /// current_version, signing the payload first. Shared by every place that
    /// mutates a seat stream (reservation, expiry, confirmation) so the
    /// append + version-bump pair can't drift between call sites.
    async fn append_event(
        &self,
        tx: &mut Transaction<'_, Postgres>,
        append: EventAppend<'_>,
        now: DateTime<Utc>,
    ) -> Result<(Uuid, String), String> {
        let EventAppend {
            stream_key,
            version,
            event_type,
            payload,
            correlation_id,
            causation_id,
        } = append;

        let metadata = json!({ "schema_version": INVENTORY_EVENT_SCHEMA_VERSION });
        let signature = self.sign(event_type, stream_key, version as u64, payload);
        let signature_bytes = hex::decode(&signature).unwrap_or_default();
        let event_uuid = Uuid::new_v4();

        sqlx::query(
            "INSERT INTO inventory.events (id, stream_key, aggregate_version, event_type, payload, metadata, correlation_id, causation_id, signature, occurred_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)"
        )
        .bind(event_uuid)
        .bind(stream_key)
        .bind(version)
        .bind(event_type)
        .bind(payload)
        .bind(&metadata)
        .bind(correlation_id)
        .bind(causation_id)
        .bind(&signature_bytes)
        .bind(now)
        .execute(&mut **tx)
        .await
        .map_err(|e| format!("Failed to insert event: {}", e))?;

        sqlx::query(
            "UPDATE inventory.event_streams SET current_version = $1, updated_at = $2 WHERE stream_key = $3"
        )
        .bind(version)
        .bind(now)
        .bind(stream_key)
        .execute(&mut **tx)
        .await
        .map_err(|e| format!("Failed to update stream: {}", e))?;

        Ok((event_uuid, signature))
    }

    async fn load_stream(
        tx: &mut Transaction<'_, Postgres>,
        stream_key: &str,
        now_ms: i64,
    ) -> Result<Option<FoldedStream>, String> {
        let stream_record = sqlx::query(
            "SELECT current_version FROM inventory.event_streams WHERE stream_key = $1 FOR UPDATE",
        )
        .bind(stream_key)
        .fetch_one(&mut **tx)
        .await;

        let current_version: i32 = match stream_record {
            Ok(r) => r.get("current_version"),
            Err(_) => return Ok(None),
        };

        let events = sqlx::query(
            "SELECT event_type, payload FROM inventory.events WHERE stream_key = $1 ORDER BY aggregate_version ASC"
        )
        .bind(stream_key)
        .fetch_all(&mut **tx)
        .await
        .map_err(|e| format!("Failed to load events: {}", e))?;

        let mut state = SeatState::default();
        for row in events {
            let event_type: String = row.get("event_type");
            let payload: serde_json::Value = row.get("payload");

            match event_type.as_str() {
                "SeatReservationHeld" => {
                    let order_id = payload
                        .get("order_id")
                        .and_then(|v| v.as_str())
                        .unwrap_or("")
                        .to_string();
                    let exp_ms = payload
                        .get("expires_at_ms")
                        .and_then(|v| v.as_i64())
                        .unwrap_or(0);
                    state.apply_reserved(order_id, exp_ms);
                }
                "SeatReservationConfirmed" => {
                    let order_id = payload
                        .get("order_id")
                        .and_then(|v| v.as_str())
                        .unwrap_or("")
                        .to_string();
                    state.apply_sold(order_id);
                }
                "SeatReservationExpired" => {
                    state.apply_expired();
                }
                _ => {}
            }
        }

        state.expire_if_due(now_ms);

        Ok(Some(FoldedStream {
            stream_key: stream_key.to_string(),
            current_version,
            state,
        }))
    }

    /// Loads the event_id/section_id/seat_id carried on a stream's latest
    /// SeatReservationHeld payload. Used wherever a later event needs to know
    /// which seat a stream refers to (expiry, confirmation).
    async fn load_held_seat_fields(
        tx: &mut Transaction<'_, Postgres>,
        stream_key: &str,
    ) -> Result<HeldSeatFields, String> {
        let held_row = sqlx::query(
            "SELECT payload FROM inventory.events WHERE stream_key = $1 AND event_type = 'SeatReservationHeld' ORDER BY aggregate_version DESC LIMIT 1"
        )
        .bind(stream_key)
        .fetch_one(&mut **tx)
        .await
        .map_err(|e| format!("Failed to load held event: {}", e))?;
        let held_payload: serde_json::Value = held_row.get("payload");
        Ok(HeldSeatFields {
            event_id: held_payload
                .get("event_id")
                .and_then(|v| v.as_str())
                .unwrap_or("")
                .to_string(),
            section_id: held_payload
                .get("section_id")
                .and_then(|v| v.as_str())
                .unwrap_or("")
                .to_string(),
            seat_id: held_payload
                .get("seat_id")
                .and_then(|v| v.as_str())
                .unwrap_or("")
                .to_string(),
        })
    }

    /// Distinct stream_keys held for an order, i.e. the seats a
    /// SeatReservationHeld was appended for under this order's correlation_id.
    /// Shared by every place that needs to act on "all seats held for order X"
    /// (payment failure, payment confirmation, expiry sweep).
    async fn held_stream_keys_for_order(
        tx: &mut Transaction<'_, Postgres>,
        order_id: &str,
    ) -> Result<Vec<String>, String> {
        Ok(sqlx::query(
            "SELECT DISTINCT stream_key FROM inventory.events WHERE correlation_id = $1 AND event_type = 'SeatReservationHeld'"
        )
        .bind(order_id)
        .fetch_all(&mut **tx)
        .await
        .map_err(|e| format!("Failed to load events: {}", e))?
        .into_iter()
        .map(|row| row.get("stream_key"))
        .collect())
    }

    pub async fn process_reservation(
        &self,
        order: &OrderCreatedPayload,
        now: DateTime<Utc>,
    ) -> Result<Vec<SeatInventoryEvent>, String> {
        let mut tx = self
            .pool
            .begin()
            .await
            .map_err(|e| format!("Failed to begin tx: {}", e))?;

        if !Self::claim_event(&mut tx, &order.outbox_event_id, "OrderCreated").await? {
            tx.rollback().await.ok();
            return Ok(Vec::new());
        }

        let mut folded = Vec::with_capacity(order.seat_ids.len());
        let mut all_available = true;

        let expires_at = now + chrono::Duration::minutes(10);
        let expires_at_ms = expires_at.timestamp_millis();

        for seat_id in &order.seat_ids {
            let stream_key = format!("seat:{}:{}:{}", order.event_id, order.section_id, seat_id);

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

            let stream =
                match Self::load_stream(&mut tx, &stream_key, now.timestamp_millis()).await? {
                    Some(s) => s,
                    None => {
                        all_available = false;
                        break;
                    }
                };

            if stream
                .state
                .can_reserve(stream.current_version as u64)
                .is_err()
            {
                all_available = false;
                break;
            }
            folded.push(stream);
        }

        if !all_available {
            let _ = tx.rollback().await;
            return Err("Seats not available".into());
        }

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

        for (i, seat_id) in order.seat_ids.iter().enumerate() {
            let stream_key = &folded[i].stream_key;
            let version = folded[i].current_version + 1;

            let payload = json!({
                "order_id": order.order_id,
                "reservation_id": order.reservation_id,
                "user_id": order.user_id,
                "seat_id": seat_id,
                "section_id": order.section_id,
                "event_id": order.event_id,
                "expires_at_ms": expires_at_ms,
            });

            let (event_uuid, signature) = self
                .append_event(
                    &mut tx,
                    EventAppend {
                        stream_key,
                        version,
                        event_type: "SeatReservationHeld",
                        payload: &payload,
                        correlation_id: &order.order_id,
                        causation_id: &order.outbox_event_id,
                    },
                    now,
                )
                .await?;

            reserved_events.push(SeatInventoryEvent {
                event_id: event_uuid.to_string(),
                aggregate_id: stream_key.clone(),
                aggregate_version: version as u64,
                event_type: "SeatReservationHeld".into(),
                correlation_id: order.order_id.clone(),
                causation_id: order.outbox_event_id.clone(),
                schema_version: INVENTORY_EVENT_SCHEMA_VERSION,
                seat: SeatDto {
                    event_id: order.event_id.clone(),
                    section_id: order.section_id.clone(),
                    seat_id: seat_id.clone(),
                    status: "HELD".into(),
                    version: version as u64,
                    expires_at_ms,
                },
                occurred_at: now,
                signature,
            });
        }

        tx.commit()
            .await
            .map_err(|e| format!("Commit failed: {}", e))?;
        Ok(reserved_events)
    }

    async fn expire_stream(
        &self,
        tx: &mut Transaction<'_, Postgres>,
        stream_key: &str,
        correlation_id: &str,
        causation_id: &str,
        now: DateTime<Utc>,
    ) -> Result<Option<SeatInventoryEvent>, String> {
        let stream = match Self::load_stream(tx, stream_key, now.timestamp_millis()).await? {
            Some(s) => s,
            None => return Ok(None),
        };

        // Compare-and-append guard: only expire a stream whose latest applied
        // event is still SeatReservationHeld. A stream already Confirmed or
        // Expired must be left untouched.
        if !matches!(stream.state.status, crate::domain::SeatStatus::Held { .. }) {
            return Ok(None);
        }

        let held = Self::load_held_seat_fields(tx, stream_key).await?;
        let version = stream.current_version + 1;
        let payload = json!({
            "order_id": correlation_id,
            "event_id": held.event_id,
            "section_id": held.section_id,
            "seat_id": held.seat_id,
        });

        let (event_uuid, signature) = self
            .append_event(
                tx,
                EventAppend {
                    stream_key,
                    version,
                    event_type: "SeatReservationExpired",
                    payload: &payload,
                    correlation_id,
                    causation_id,
                },
                now,
            )
            .await?;

        Ok(Some(SeatInventoryEvent {
            event_id: event_uuid.to_string(),
            aggregate_id: stream_key.to_string(),
            aggregate_version: version as u64,
            event_type: "SeatReservationExpired".into(),
            correlation_id: correlation_id.to_string(),
            causation_id: causation_id.to_string(),
            schema_version: INVENTORY_EVENT_SCHEMA_VERSION,
            seat: SeatDto {
                event_id: held.event_id,
                section_id: held.section_id,
                seat_id: held.seat_id,
                status: "AVAILABLE".into(),
                version: version as u64,
                expires_at_ms: 0,
            },
            occurred_at: now,
            signature,
        }))
    }

    pub async fn process_payment_failed(
        &self,
        order_id: &str,
        causation_event_id: &str,
        now: DateTime<Utc>,
    ) -> Result<Vec<SeatInventoryEvent>, String> {
        let mut tx = self
            .pool
            .begin()
            .await
            .map_err(|e| format!("Failed to begin tx: {}", e))?;

        if !Self::claim_event(&mut tx, causation_event_id, "PaymentFailed").await? {
            tx.rollback().await.ok();
            return Ok(Vec::new());
        }

        let stream_keys = Self::held_stream_keys_for_order(&mut tx, order_id).await?;

        let mut expired_events = Vec::new();
        for stream_key in &stream_keys {
            if let Some(event) = self
                .expire_stream(&mut tx, stream_key, order_id, causation_event_id, now)
                .await?
            {
                expired_events.push(event);
            }
        }

        let order_uuid = match Uuid::parse_str(order_id) {
            Ok(u) => u,
            Err(_) => return Err("Invalid order_id UUID".into()),
        };
        let _ =
            sqlx::query("UPDATE inventory.reservations SET status = 'EXPIRED' WHERE order_id = $1")
                .bind(order_uuid)
                .execute(&mut *tx)
                .await;

        tx.commit()
            .await
            .map_err(|e| format!("Commit failed: {}", e))?;
        Ok(expired_events)
    }

    /// Confirms all held seats for an order after payment succeeds. Seats whose
    /// hold already expired are skipped (rejected) rather than confirmed, since
    /// there is no later valid hold to honor.
    pub async fn process_payment_confirmed(
        &self,
        order_id: &str,
        causation_event_id: &str,
        now: DateTime<Utc>,
    ) -> Result<Vec<SeatInventoryEvent>, String> {
        let mut tx = self
            .pool
            .begin()
            .await
            .map_err(|e| format!("Failed to begin tx: {}", e))?;

        if !Self::claim_event(&mut tx, causation_event_id, "OrderConfirmed").await? {
            tx.rollback().await.ok();
            return Ok(Vec::new());
        }

        let stream_keys = Self::held_stream_keys_for_order(&mut tx, order_id).await?;

        let mut confirmed_events = Vec::new();
        for stream_key in &stream_keys {
            let stream =
                match Self::load_stream(&mut tx, stream_key, now.timestamp_millis()).await? {
                    Some(s) => s,
                    None => continue,
                };
            if !matches!(stream.state.status, crate::domain::SeatStatus::Held { .. }) {
                tracing::warn!(
                    order_id,
                    stream_key,
                    "hold no longer active, skipping confirmation"
                );
                continue;
            }

            let held = Self::load_held_seat_fields(&mut tx, stream_key).await?;
            let version = stream.current_version + 1;
            let payload = json!({
                "order_id": order_id,
                "event_id": held.event_id,
                "section_id": held.section_id,
                "seat_id": held.seat_id,
            });

            let (event_uuid, signature) = self
                .append_event(
                    &mut tx,
                    EventAppend {
                        stream_key,
                        version,
                        event_type: "SeatReservationConfirmed",
                        payload: &payload,
                        correlation_id: order_id,
                        causation_id: causation_event_id,
                    },
                    now,
                )
                .await?;

            confirmed_events.push(SeatInventoryEvent {
                event_id: event_uuid.to_string(),
                aggregate_id: stream_key.clone(),
                aggregate_version: version as u64,
                event_type: "SeatReservationConfirmed".into(),
                correlation_id: order_id.to_string(),
                causation_id: causation_event_id.to_string(),
                schema_version: INVENTORY_EVENT_SCHEMA_VERSION,
                seat: SeatDto {
                    event_id: held.event_id,
                    section_id: held.section_id,
                    seat_id: held.seat_id,
                    status: "SOLD".into(),
                    version: version as u64,
                    expires_at_ms: 0,
                },
                occurred_at: now,
                signature,
            });
        }

        let order_uuid = match Uuid::parse_str(order_id) {
            Ok(u) => u,
            Err(_) => return Err("Invalid order_id UUID".into()),
        };
        let _ = sqlx::query(
            "UPDATE inventory.reservations SET status = 'CONFIRMED' WHERE order_id = $1",
        )
        .bind(order_uuid)
        .execute(&mut *tx)
        .await;

        tx.commit()
            .await
            .map_err(|e| format!("Commit failed: {}", e))?;
        Ok(confirmed_events)
    }

    /// Background sweep: expires held reservations whose deadline has passed
    /// with no follow-up payment outcome. Bounded to a fixed batch per call.
    pub async fn expire_due_reservations(
        &self,
        now: DateTime<Utc>,
    ) -> Result<Vec<SeatInventoryEvent>, String> {
        let mut tx = self
            .pool
            .begin()
            .await
            .map_err(|e| format!("Failed to begin tx: {}", e))?;

        let due: Vec<(String, Uuid)> = sqlx::query(
            "SELECT reservation_id, order_id FROM inventory.reservations WHERE status = 'HELD' AND expires_at <= $1 FOR UPDATE SKIP LOCKED LIMIT $2"
        )
        .bind(now)
        .bind(MAX_EXPIRING_RESERVATIONS_PER_SWEEP)
        .fetch_all(&mut *tx)
        .await
        .map_err(|e| format!("Failed to load due reservations: {}", e))?
        .into_iter()
        .map(|row| (row.get("reservation_id"), row.get("order_id")))
        .collect();

        let mut expired_events = Vec::new();
        for (_reservation_id, order_uuid) in &due {
            let order_id = order_uuid.to_string();
            let stream_keys = Self::held_stream_keys_for_order(&mut tx, &order_id).await?;

            for stream_key in &stream_keys {
                if let Some(event) = self
                    .expire_stream(
                        &mut tx,
                        stream_key,
                        &order_id,
                        EXPIRY_SCHEDULER_CAUSATION_ID,
                        now,
                    )
                    .await?
                {
                    expired_events.push(event);
                }
            }

            let _ = sqlx::query(
                "UPDATE inventory.reservations SET status = 'EXPIRED' WHERE order_id = $1 AND status = 'HELD'",
            )
            .bind(order_uuid)
            .execute(&mut *tx)
            .await;
        }

        tx.commit()
            .await
            .map_err(|e| format!("Commit failed: {}", e))?;
        Ok(expired_events)
    }

    /// Builds one SeatReservationFailed event per requested seat when a whole
    /// reservation attempt is rejected atomically (no seats were held).
    pub fn build_reservation_failed_events(
        &self,
        order: &OrderCreatedPayload,
        reason: &str,
        now: DateTime<Utc>,
    ) -> Vec<SeatReservationFailedEvent> {
        order
            .seat_ids
            .iter()
            .map(|seat_id| {
                let stream_key =
                    format!("seat:{}:{}:{}", order.event_id, order.section_id, seat_id);
                let payload = json!({ "order_id": order.order_id, "reason": reason });
                let signature = self.sign("SeatReservationFailed", &stream_key, 0, &payload);
                SeatReservationFailedEvent {
                    event_id: Uuid::new_v4().to_string(),
                    aggregate_id: stream_key,
                    event_type: "SeatReservationFailed".into(),
                    correlation_id: order.order_id.clone(),
                    causation_id: order.outbox_event_id.clone(),
                    schema_version: INVENTORY_EVENT_SCHEMA_VERSION,
                    order_id: order.order_id.clone(),
                    reason: reason.to_string(),
                    occurred_at: now,
                    signature,
                }
            })
            .collect()
    }
}
