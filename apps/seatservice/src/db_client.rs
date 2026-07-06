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
/// Bounds how many seat streams process_event_cancelled locks and processes
/// per transaction, mirroring MAX_EXPIRING_RESERVATIONS_PER_SWEEP. Without
/// this, cancelling one large venue's event would `FOR UPDATE` every one of
/// its seat streams up front and hold those locks for the whole sequential
/// fan-out.
const MAX_CANCELLING_STREAMS_PER_BATCH: i64 = 500;

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

/// The event_id/section_id/seat_id fields carried on every
/// SeatReservationHeld payload, re-extracted whenever a later event (expiry,
/// confirmation) needs to know which seat a stream refers to. Both callers
/// (expire_stream, process_reservation_confirmed) derive the owning order_id
/// from their own order_id parameter instead of from this payload.
struct HeldSeatFields {
    event_id: String,
    section_id: String,
    seat_id: String,
}

/// The event_id/section_id/seat_id captured on inventory.event_streams at
/// event-creation time (apigateway's CreateEvent - see migration
/// 008_event_cancellation.sql for why a cancelled seat must never look
/// rebookable). Unlike HeldSeatFields, which is read from a stream's
/// historical SeatReservationHeld event, these columns are always present for
/// every stream, whether or not it was ever held - which is what lets
/// cancel_stream cancel a virgin (never-touched) stream.
struct StreamSeatIdentity {
    stream_key: String,
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
                "SeatReservationCancelled" => {
                    let order_id = payload
                        .get("order_id")
                        .and_then(|v| v.as_str())
                        .unwrap_or("")
                        .to_string();
                    state.apply_cancelled(order_id);
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
    /// which seat a stream refers to (expiry, confirmation) - both callers
    /// only ever invoke this for a stream already confirmed to be Held, so a
    /// SeatReservationHeld event is guaranteed to exist.
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

    /// Best-effort recovery of a stream's real owning order_id from its
    /// historical SeatReservationHeld event, tolerant of there being none.
    /// Unlike load_held_seat_fields - used by expire_stream and
    /// process_reservation_confirmed, which only ever look at a stream
    /// already confirmed to be Held, so a SeatReservationHeld event is
    /// guaranteed to exist - this is called from cancel_stream, which also
    /// fires for a virgin stream that was pre-seeded at event creation but
    /// never held. A seat that was never held legitimately has no owning
    /// order, so this returns "" rather than erroring in that case.
    async fn load_owning_order_id(
        tx: &mut Transaction<'_, Postgres>,
        stream_key: &str,
    ) -> Result<String, String> {
        let held_row = sqlx::query(
            "SELECT payload FROM inventory.events WHERE stream_key = $1 AND event_type = 'SeatReservationHeld' ORDER BY aggregate_version DESC LIMIT 1"
        )
        .bind(stream_key)
        .fetch_optional(&mut **tx)
        .await
        .map_err(|e| format!("Failed to load held event: {}", e))?;

        Ok(held_row
            .and_then(|row| {
                let payload: serde_json::Value = row.get("payload");
                payload
                    .get("order_id")
                    .and_then(|v| v.as_str())
                    .map(|s| s.to_string())
            })
            .unwrap_or_default())
    }

    /// Builds a SeatReservationCancelled payload for cancel_stream. The
    /// "order_id" field must be the seat's real owning order (recovered from
    /// its SeatReservationHeld history via load_owning_order_id, or "" for a
    /// virgin seat that was never held) - never the correlation_id/causation_id
    /// used for event-wide cancellation bookkeeping, which for
    /// process_event_cancelled is the catalog event_id rather than any single
    /// order.
    fn cancelled_payload(order_id: &str, seat: &StreamSeatIdentity) -> serde_json::Value {
        json!({
            "order_id": order_id,
            "event_id": seat.event_id,
            "section_id": seat.section_id,
            "seat_id": seat.seat_id,
        })
    }

    /// Distinct stream_keys held for an order, i.e. the seats a
    /// SeatReservationHeld was appended for under this order's correlation_id.
    /// Shared by every place that needs to act on "all seats held for order X"
    /// (reservation cancellation, reservation confirmation, expiry sweep).
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

    /// Cancels a stream - virgin, Held, Sold, or Available-after-having-been-
    /// touched - because the organizer cancelled the whole event. Unlike
    /// expire_stream, this fires from any non-terminal prior state, including
    /// a virgin stream that was never held, and lands on a terminal Cancelled
    /// status rather than Available, so a cancelled event's seats never look
    /// rebookable. `seat` supplies the event_id/section_id/seat_id from
    /// inventory.event_streams directly (always present) rather than from a
    /// SeatReservationHeld event (which a virgin stream doesn't have).
    async fn cancel_stream(
        &self,
        tx: &mut Transaction<'_, Postgres>,
        seat: &StreamSeatIdentity,
        correlation_id: &str,
        causation_id: &str,
        now: DateTime<Utc>,
    ) -> Result<Option<SeatInventoryEvent>, String> {
        let stream_key = seat.stream_key.as_str();
        let stream = match Self::load_stream(tx, stream_key, now.timestamp_millis()).await? {
            Some(s) => s,
            None => return Ok(None),
        };

        // Compare-and-append guard: see should_skip_cancellation - only an
        // already-Cancelled stream (terminal) is skipped. This fires even for
        // a virgin stream never held, or one that raced back to Available via
        // expire_stream after being Held, so the event's seats never look
        // rebookable.
        if crate::domain::should_skip_cancellation(&stream.state.status) {
            return Ok(None);
        }

        let order_id = Self::load_owning_order_id(tx, stream_key).await?;
        let version = stream.current_version + 1;
        let payload = Self::cancelled_payload(&order_id, seat);

        let (event_uuid, signature) = self
            .append_event(
                tx,
                EventAppend {
                    stream_key,
                    version,
                    event_type: "SeatReservationCancelled",
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
            event_type: "SeatReservationCancelled".into(),
            correlation_id: correlation_id.to_string(),
            causation_id: causation_id.to_string(),
            schema_version: INVENTORY_EVENT_SCHEMA_VERSION,
            seat: SeatDto {
                event_id: seat.event_id.clone(),
                section_id: seat.section_id.clone(),
                seat_id: seat.seat_id.clone(),
                status: "CANCELLED".into(),
                version: version as u64,
                expires_at_ms: 0,
            },
            occurred_at: now,
            signature,
        }))
    }

    /// Expires all held seats for an order because the user or an external
    /// process explicitly cancelled a held reservation.
    pub async fn process_reservation_cancelled(
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

        if !Self::claim_event(&mut tx, causation_event_id, "OrderCancelled").await? {
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

    /// Confirms all held seats for an order after the user explicitly confirmed
    /// a held reservation. Seats whose hold already expired are skipped
    /// (rejected) rather than confirmed, since there is no later valid hold to
    /// honor; a SeatReservationConfirmationFailed event is published for each
    /// rejected seat so orderservice can correct an order that already went
    /// CONFIRMED locally before consuming the earlier expiry.
    pub async fn process_reservation_confirmed(
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

            // Loaded up front (rather than only in the Held branch below) so the
            // same lookup can back both outcomes: appending SeatReservationConfirmed,
            // or - if the hold already lost the race against an independent expiry -
            // building the SeatReservationConfirmationFailed compensating event below.
            let held = Self::load_held_seat_fields(&mut tx, stream_key).await?;

            if !matches!(stream.state.status, crate::domain::SeatStatus::Held { .. }) {
                // Compare-and-append guard: the hold already expired (e.g. lost
                // the race against seatservice's own expiry sweep) before this
                // OrderConfirmed arrived, so this stream must NOT have
                // SeatReservationConfirmed appended onto it - it's already
                // Expired. But orderservice's locally-mirrored order may still
                // be advancing to CONFIRMED for this order, so publish a
                // compensating, non-stream-mutating event that tells
                // orderservice to correct itself back to EXPIRED instead of
                // being stuck CONFIRMED with no ticket.
                tracing::warn!(
                    order_id,
                    stream_key,
                    "hold no longer active, skipping confirmation"
                );
                let payload = json!({
                    "order_id": order_id,
                    "event_id": held.event_id,
                    "section_id": held.section_id,
                    "seat_id": held.seat_id,
                });
                let signature = self.sign(
                    "SeatReservationConfirmationFailed",
                    stream_key,
                    stream.current_version as u64,
                    &payload,
                );
                confirmed_events.push(SeatInventoryEvent {
                    event_id: Uuid::new_v4().to_string(),
                    aggregate_id: stream_key.clone(),
                    aggregate_version: stream.current_version as u64,
                    event_type: "SeatReservationConfirmationFailed".into(),
                    correlation_id: order_id.to_string(),
                    causation_id: causation_event_id.to_string(),
                    schema_version: INVENTORY_EVENT_SCHEMA_VERSION,
                    seat: SeatDto {
                        event_id: held.event_id,
                        section_id: held.section_id,
                        seat_id: held.seat_id,
                        status: "EXPIRED".into(),
                        version: stream.current_version as u64,
                        expires_at_ms: 0,
                    },
                    occurred_at: now,
                    signature,
                });
                continue;
            }

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

    /// Processes one bounded batch (at most MAX_CANCELLING_STREAMS_PER_BATCH)
    /// of not-yet-cancelled seat streams for an event, in its own
    /// transaction. The `NOT EXISTS` clause - rather than a folded-status
    /// filter - is what makes this safe to call repeatedly: it only ever
    /// selects streams that truly have no SeatReservationCancelled event yet,
    /// so a stream finished by an earlier batch is never re-selected (aside
    /// from the deliberate `skip_locked = false` mop-up below), and the
    /// caller can tell "more remain" from "done" by comparing the returned
    /// row count against the batch limit.
    async fn process_event_cancelled_batch(
        &self,
        event_id: &str,
        causation_event_id: &str,
        now: DateTime<Utc>,
        skip_locked: bool,
    ) -> Result<(Vec<SeatInventoryEvent>, usize), String> {
        let mut tx = self
            .pool
            .begin()
            .await
            .map_err(|e| format!("Failed to begin tx: {}", e))?;

        // Two literal query strings (rather than interpolating the lock
        // clause into one dynamic string) so sqlx's compile-time check for
        // ad hoc SQL string building still applies - skip_locked is an
        // internal bool, never user input, but there's no need to opt out of
        // that check when the query text can just as easily stay static.
        let rows = if skip_locked {
            sqlx::query(
                "SELECT stream_key, section_id, seat_id FROM inventory.event_streams \
                 WHERE event_id = $1 AND NOT EXISTS ( \
                     SELECT 1 FROM inventory.events e \
                     WHERE e.stream_key = inventory.event_streams.stream_key \
                       AND e.event_type = 'SeatReservationCancelled' \
                 ) FOR UPDATE SKIP LOCKED LIMIT $2",
            )
            .bind(event_id)
            .bind(MAX_CANCELLING_STREAMS_PER_BATCH)
            .fetch_all(&mut *tx)
            .await
        } else {
            sqlx::query(
                "SELECT stream_key, section_id, seat_id FROM inventory.event_streams \
                 WHERE event_id = $1 AND NOT EXISTS ( \
                     SELECT 1 FROM inventory.events e \
                     WHERE e.stream_key = inventory.event_streams.stream_key \
                       AND e.event_type = 'SeatReservationCancelled' \
                 ) FOR UPDATE LIMIT $2",
            )
            .bind(event_id)
            .bind(MAX_CANCELLING_STREAMS_PER_BATCH)
            .fetch_all(&mut *tx)
            .await
        }
        .map_err(|e| format!("Failed to load event streams: {}", e))?;

        let rows_seen = rows.len();
        let mut cancelled_events = Vec::new();
        for row in rows {
            let seat = StreamSeatIdentity {
                stream_key: row.get("stream_key"),
                event_id: event_id.to_string(),
                section_id: row.get("section_id"),
                seat_id: row.get("seat_id"),
            };
            if let Some(event) = self
                .cancel_stream(&mut tx, &seat, event_id, causation_event_id, now)
                .await?
            {
                cancelled_events.push(event);
            }
        }

        tx.commit()
            .await
            .map_err(|e| format!("Commit failed: {}", e))?;
        Ok((cancelled_events, rows_seen))
    }

    /// Cancels every seat stream belonging to an event because the organizer
    /// cancelled the whole event. Fans out across all streams tagged with
    /// this catalog event_id, regardless of which order (if any) currently
    /// holds or has bought each seat.
    ///
    /// Bounded batching: rather than one giant transaction that locks and
    /// walks every seat stream for the event (unbounded lock duration for a
    /// large venue), this processes MAX_CANCELLING_STREAMS_PER_BATCH streams
    /// per transaction, looping until a batch comes back short of the limit -
    /// which, thanks to the batch query's `NOT EXISTS (SeatReservationCancelled)`
    /// filter, only happens once every stream has genuinely been cancelled.
    ///
    /// Dedup/crash-safety design: the EventCancelled message is still claimed
    /// via claim_event (for parity with every other handler and for
    /// observability of "have we ever seen this event_id"), but that claim
    /// does NOT gate whether the batch loop below runs. If it did, a consumer
    /// crash after the claim succeeded but before every batch had run would
    /// mean a Kafka redelivery of the same message sees "already claimed" and
    /// returns immediately - silently abandoning whatever streams hadn't been
    /// cancelled yet, with nothing left to ever retry them. Instead,
    /// correctness/idempotency comes from each batch's own `NOT EXISTS` filter
    /// and from cancel_stream's per-stream Cancelled guard: re-running this
    /// whole function for an event that's already fully cancelled is a cheap,
    /// harmless no-op (every batch immediately returns 0 rows), so it's safe
    /// to always run the loop to completion on every delivery or redelivery.
    pub async fn process_event_cancelled(
        &self,
        event_id: &str,
        causation_event_id: &str,
        now: DateTime<Utc>,
    ) -> Result<Vec<SeatInventoryEvent>, String> {
        {
            let mut claim_tx = self
                .pool
                .begin()
                .await
                .map_err(|e| format!("Failed to begin tx: {}", e))?;
            Self::claim_event(&mut claim_tx, causation_event_id, "EventCancelled").await?;
            claim_tx
                .commit()
                .await
                .map_err(|e| format!("Commit failed: {}", e))?;
        }

        let mut cancelled_events = Vec::new();

        // Fast path: SKIP LOCKED batches avoid blocking on - or being blocked
        // by - an unrelated concurrent transaction (e.g. a customer's
        // in-flight reservation attempt) that happens to hold a lock on one
        // of these streams. Loop until a batch returns fewer rows than the
        // limit, which (thanks to the NOT EXISTS filter re-evaluating on
        // every iteration) means no not-yet-cancelled, currently-unlocked
        // stream remains.
        loop {
            let (events, rows_seen) = self
                .process_event_cancelled_batch(event_id, causation_event_id, now, true)
                .await?;
            cancelled_events.extend(events);
            if (rows_seen as i64) < MAX_CANCELLING_STREAMS_PER_BATCH {
                break;
            }
        }

        // Mop-up pass: a stream that was transiently locked out of every
        // SKIP LOCKED batch above (e.g. its one unlucky moment of contention
        // landed on the final, under-the-limit batch) would otherwise be
        // permanently missed, since nothing else revisits this event once
        // process_event_cancelled returns. This blocks briefly on any such
        // straggler instead of skipping it; it should almost always observe
        // zero rows and return immediately.
        loop {
            let (events, rows_seen) = self
                .process_event_cancelled_batch(event_id, causation_event_id, now, false)
                .await?;
            cancelled_events.extend(events);
            if (rows_seen as i64) < MAX_CANCELLING_STREAMS_PER_BATCH {
                break;
            }
        }

        Ok(cancelled_events)
    }

    /// Background sweep: expires held reservations whose deadline has passed
    /// with no follow-up confirmation or cancellation. Bounded to a fixed batch
    /// per call.
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

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn cancelled_payload_uses_real_order_id_not_catalog_event_id() {
        // Event-wide cancellation uses catalog event_id as correlation_id, so
        // order_id must come from the seat's own held history.
        let seat = StreamSeatIdentity {
            stream_key: "seat:catalog-event-999:sec-A:seat-12".into(),
            event_id: "catalog-event-999".into(),
            section_id: "sec-A".into(),
            seat_id: "seat-12".into(),
        };

        let payload = DbClient::cancelled_payload("order-real-123", &seat);

        assert_eq!(payload["order_id"], "order-real-123");
        assert_ne!(payload["order_id"], "catalog-event-999");
        assert_eq!(payload["event_id"], "catalog-event-999");
        assert_eq!(payload["section_id"], "sec-A");
        assert_eq!(payload["seat_id"], "seat-12");
    }

    #[test]
    fn cancelled_payload_uses_empty_order_id_for_virgin_seat() {
        // Virgin seats have no owning order; preserve "" instead of inventing one.
        let seat = StreamSeatIdentity {
            stream_key: "seat:catalog-event-999:sec-A:seat-99".into(),
            event_id: "catalog-event-999".into(),
            section_id: "sec-A".into(),
            seat_id: "seat-99".into(),
        };

        let payload = DbClient::cancelled_payload("", &seat);

        assert_eq!(payload["order_id"], "");
        assert_eq!(payload["event_id"], "catalog-event-999");
        assert_eq!(payload["section_id"], "sec-A");
        assert_eq!(payload["seat_id"], "seat-99");
    }

    // These process_event_cancelled/cancel_stream behaviors need live sqlx
    // queries, and this crate has no live-DB integration harness yet:
    //   - a virgin seat stream (current_version == 0, no SeatReservationHeld
    //     event) is cancelled with an empty "" order_id in its
    //     SeatReservationCancelled payload;
    //   - a touched-then-available (raced) seat stream is cancelled with its
    //     real order_id recovered from its SeatReservationHeld history;
    //   - a Held or Sold seat stream is cancelled normally;
    //   - an already-Cancelled seat stream is skipped (no duplicate event);
    //   - process_event_cancelled's batching/pagination processes more
    //     streams than a single batch's MAX_CANCELLING_STREAMS_PER_BATCH
    //     limit across multiple transactions.
}
