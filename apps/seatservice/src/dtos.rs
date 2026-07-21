use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};

pub const INVENTORY_EVENT_SCHEMA_VERSION: u32 = 1;

#[derive(Debug, Deserialize)]
pub struct EventEnvelope {
    #[serde(
        alias = "EventType",
        alias = "event_type",
        alias = "Type",
        alias = "type"
    )]
    pub event_type: String,
    #[serde(alias = "Payload", alias = "payload", alias = "Order", alias = "order")]
    pub payload: Option<serde_json::Value>,
}

#[derive(Debug, Deserialize)]
pub struct OrderCreatedPayload {
    #[serde(alias = "OutboxEventID", alias = "outbox_event_id")]
    pub outbox_event_id: String,
    #[serde(alias = "OrderID", alias = "id", alias = "ID")]
    pub order_id: String,
    #[serde(alias = "UserID", alias = "user_id", alias = "UserId")]
    pub user_id: String,
    #[serde(alias = "EventID", alias = "event_id", alias = "EventId")]
    pub event_id: String,
    #[serde(alias = "SectionID", alias = "section_id", alias = "SectionId")]
    pub section_id: String,
    #[serde(
        alias = "ReservationID",
        alias = "reservation_id",
        alias = "ReservationId"
    )]
    pub reservation_id: String,
    #[serde(alias = "SeatIDs", alias = "seat_ids", alias = "SeatIds")]
    pub seat_ids: Vec<String>,
}

/// Emitted for every seatservice-owned inventory transition (held, expired,
/// confirmed). `event_type` carries the specific transition name.
#[derive(Debug, Serialize)]
pub struct SeatInventoryEvent {
    pub event_id: String,
    pub aggregate_id: String,
    pub aggregate_version: u64,
    #[serde(rename = "type")]
    pub event_type: String,
    pub correlation_id: String,
    pub causation_id: String,
    pub schema_version: u32,
    pub seat: SeatDto,
    pub occurred_at: DateTime<Utc>,
    pub signature: String,
}

#[derive(Debug, Serialize)]
pub struct SeatDto {
    pub event_id: String,
    pub section_id: String,
    pub seat_id: String,
    pub status: String, // "HELD" | "AVAILABLE" | "RESERVED" | "CANCELLED"
    pub version: u64,
    pub expires_at_ms: i64,
}

#[derive(Debug, Serialize)]
pub struct SeatReservationFailedEvent {
    pub event_id: String,
    pub aggregate_id: String,
    #[serde(rename = "type")]
    pub event_type: String, // "SeatReservationFailed"
    pub correlation_id: String,
    pub causation_id: String,
    pub schema_version: u32,
    pub order_id: String,
    pub reason: String,
    pub occurred_at: DateTime<Utc>,
    pub signature: String,
}
