use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};

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

#[derive(Debug, Serialize)]
pub struct SeatReservedEvent {
    pub event_id: String,
    pub aggregate_id: String,
    pub aggregate_version: u64,
    #[serde(rename = "type")]
    pub event_type: String, // "SeatReserved"
    pub seat: SeatDto,
    pub occurred_at: DateTime<Utc>,
}

#[derive(Debug, Serialize)]
pub struct SeatDto {
    pub event_id: String,
    pub section_id: String,
    pub seat_id: String,
    pub status: String, // "HELD"
    pub version: u64,
    pub expires_at_ms: i64,
}

#[derive(Debug, Serialize)]
pub struct SeatReservationFailedEvent {
    #[serde(rename = "type")]
    pub event_type: String, // "SeatReservationFailed"
    pub order_id: String,
    pub reason: String,
}
