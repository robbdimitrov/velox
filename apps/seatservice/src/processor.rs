use crate::db_client::DbClient;
use crate::dtos::{EventEnvelope, OrderCreatedPayload};
use chrono::Utc;
use rdkafka::producer::{FutureProducer, FutureRecord};
use sha2::{Digest, Sha256};
use std::time::Duration;
use tracing::Instrument;
use tracing::{error, info, warn};

const INVENTORY_TOPIC: &str = "inventory.events.v1";
const DLQ_TOPIC: &str = "dlq.order.events.v1";
const CONSUMER_GROUP: &str = "seatservice_group";

pub struct MessageMeta {
    pub source_partition: i32,
    pub source_offset: i64,
    pub request_id: Option<String>,
}

async fn send_to_dlq(
    producer: &FutureProducer,
    meta: &MessageMeta,
    payload_bytes: &[u8],
    error_class: &str,
    error_message: &str,
) {
    let payload_hash = hex::encode(Sha256::digest(payload_bytes));
    let now = Utc::now();
    let record_value = serde_json::json!({
        "source_topic": "order.events.v1",
        "source_partition": meta.source_partition,
        "source_offset": meta.source_offset,
        "consumer_group": CONSUMER_GROUP,
        "error_class": error_class,
        "error_message": error_message,
        "payload_hash": payload_hash,
        "first_seen_at": now,
        "last_seen_at": now,
        "correlation_id": meta.request_id.clone().unwrap_or_default(),
    });
    let Ok(msg_str) = serde_json::to_string(&record_value) else {
        error!("failed to serialize DLQ record");
        return;
    };
    let record = FutureRecord::to(DLQ_TOPIC)
        .payload(&msg_str)
        .key(error_class);
    if let Err((e, _)) = producer.send(record, Duration::from_secs(5)).await {
        error!(error = %e, "failed to publish DLQ record");
    }
}

/// Returns whether the Kafka offset may be committed: true for durable handling
/// or DLQ, false for transient failures that need redelivery.
pub async fn process_message(
    db: &DbClient,
    producer: &FutureProducer,
    payload_bytes: &[u8],
    meta: MessageMeta,
) -> bool {
    let request_id = meta.request_id.clone();
    let req_id_str = request_id.clone().unwrap_or_else(|| "unknown".to_string());
    let span = tracing::info_span!("process_message", request_id = %req_id_str);

    async move {
        let envelope: EventEnvelope = match serde_json::from_slice(payload_bytes) {
            Ok(e) => e,
            Err(err) => {
                warn!(error = %err, "Failed to deserialize event envelope");
                send_to_dlq(
                    producer,
                    &meta,
                    payload_bytes,
                    "envelope_deserialize_error",
                    &err.to_string(),
                )
                .await;
                return true;
            }
        };

        if envelope.event_type == "OrderCreated" {
            let payload_val = match envelope.payload {
                Some(p) => p,
                None => {
                    warn!("OrderCreated missing payload");
                    send_to_dlq(producer, &meta, payload_bytes, "missing_payload", "OrderCreated missing payload").await;
                    return true;
                }
            };

            let order: OrderCreatedPayload = match serde_json::from_value(payload_val) {
                Ok(o) => o,
                Err(err) => {
                    warn!(error = %err, "Failed to deserialize OrderCreated payload");
                    send_to_dlq(producer, &meta, payload_bytes, "payload_deserialize_error", &err.to_string()).await;
                    return true;
                }
            };

            if order.order_id.is_empty()
                || order.event_id.is_empty()
                || order.section_id.is_empty()
                || order.seat_ids.is_empty()
                || order.reservation_id.is_empty()
                || order.outbox_event_id.is_empty()
            {
                warn!(order_id = %order.order_id, "Missing required fields in OrderCreated");
                send_to_dlq(producer, &meta, payload_bytes, "missing_required_fields", "OrderCreated missing required fields").await;
                return true;
            }

            info!(order_id = %order.order_id, "Processing OrderCreated");

            let now = Utc::now();
            match db.process_reservation(&order, now).await {
                Ok(reserved_events) => {
                    let mut published = true;
                    for event in reserved_events {
                        published &= publish(producer, &event.event_type.clone(), &event.aggregate_id.clone(), &event, request_id.as_deref()).await;
                    }
                    info!(order_id = %order.order_id, count = order.seat_ids.len(), "Successfully reserved seats");
                    if !published {
                        return false;
                    }
                }
                Err(reason) => {
                    warn!(order_id = %order.order_id, reason = %reason, "Failed to reserve seats");
                    let mut published = true;
                    for failed_event in db.build_reservation_failed_events(&order, &reason, now) {
                        let aggregate_id = failed_event.aggregate_id.clone();
                        published &= publish(producer, "SeatReservationFailed", &aggregate_id, &failed_event, request_id.as_deref()).await;
                    }
                    if !published {
                        return false;
                    }
                }
            }
            true
        } else if envelope.event_type == "OrderCancelled" {
            let Some(payload_val) = envelope.payload else { return true };
            let order_id = payload_val.get("order_id").and_then(|v| v.as_str()).unwrap_or("").to_string();
            let outbox_event_id = payload_val.get("outbox_event_id").and_then(|v| v.as_str()).unwrap_or("").to_string();
            if order_id.is_empty() || outbox_event_id.is_empty() {
                warn!("OrderCancelled missing order_id or outbox_event_id");
                send_to_dlq(producer, &meta, payload_bytes, "missing_required_fields", "OrderCancelled missing required fields").await;
                return true;
            }

            info!(order_id = %order_id, "Processing OrderCancelled");
            match db.process_reservation_cancelled(&order_id, &outbox_event_id, Utc::now()).await {
                Ok(expired_events) => {
                    let mut published = true;
                    for event in expired_events {
                        published &= publish(producer, &event.event_type.clone(), &event.aggregate_id.clone(), &event, request_id.as_deref()).await;
                    }
                    published
                }
                Err(e) => {
                    warn!("Failed to process reservation cancellation: {}", e);
                    false
                }
            }
        } else if envelope.event_type == "OrderConfirmed" {
            let Some(payload_val) = envelope.payload else { return true };
            let order_id = payload_val.get("order_id").and_then(|v| v.as_str()).unwrap_or("").to_string();
            let outbox_event_id = payload_val.get("outbox_event_id").and_then(|v| v.as_str()).unwrap_or("").to_string();
            if order_id.is_empty() || outbox_event_id.is_empty() {
                warn!("OrderConfirmed missing order_id or outbox_event_id");
                send_to_dlq(producer, &meta, payload_bytes, "missing_required_fields", "OrderConfirmed missing required fields").await;
                return true;
            }

            info!(order_id = %order_id, "Processing OrderConfirmed");
            match db.process_reservation_confirmed(&order_id, &outbox_event_id, Utc::now()).await {
                Ok(confirmed_events) => {
                    let mut published = true;
                    for event in confirmed_events {
                        published &= publish(producer, &event.event_type.clone(), &event.aggregate_id.clone(), &event, request_id.as_deref()).await;
                    }
                    published
                }
                Err(e) => {
                    warn!("Failed to process reservation confirmation: {}", e);
                    false
                }
            }
        } else if envelope.event_type == "EventCancelled" {
            let Some(payload_val) = envelope.payload else { return true };
            let event_id = payload_val.get("event_id").and_then(|v| v.as_str()).unwrap_or("").to_string();
            let outbox_event_id = payload_val.get("outbox_event_id").and_then(|v| v.as_str()).unwrap_or("").to_string();
            if event_id.is_empty() || outbox_event_id.is_empty() {
                warn!("EventCancelled missing event_id or outbox_event_id");
                send_to_dlq(producer, &meta, payload_bytes, "missing_required_fields", "EventCancelled missing required fields").await;
                return true;
            }

            info!(event_id = %event_id, "Processing EventCancelled");
            match db.process_event_cancelled(&event_id, &outbox_event_id, Utc::now()).await {
                Ok(cancelled_events) => {
                    let mut published = true;
                    for event in cancelled_events {
                        published &= publish(producer, &event.event_type.clone(), &event.aggregate_id.clone(), &event, request_id.as_deref()).await;
                    }
                    published
                }
                Err(e) => {
                    warn!("Failed to process event cancellation: {}", e);
                    false
                }
            }
        } else {
            true
        }
    }
    .instrument(span)
    .await
}

pub(crate) async fn publish<T: serde::Serialize>(
    producer: &FutureProducer,
    event_type: &str,
    key: &str,
    event: &T,
    request_id: Option<&str>,
) -> bool {
    let msg_str = match serde_json::to_string(event) {
        Ok(s) => s,
        Err(e) => {
            error!(error = %e, event_type, "Failed to serialize event");
            return false;
        }
    };

    let mut record = FutureRecord::to(INVENTORY_TOPIC).payload(&msg_str).key(key);
    let mut headers = rdkafka::message::OwnedHeaders::new();
    headers = headers.insert(rdkafka::message::Header {
        key: "event_type",
        value: Some(event_type),
    });
    if let Some(rid) = request_id {
        headers = headers.insert(rdkafka::message::Header {
            key: "X-Request-ID",
            value: Some(rid),
        });
    }
    record = record.headers(headers);

    if let Err((e, _)) = producer.send(record, Duration::from_secs(5)).await {
        error!(error = %e, event_type, "Failed to produce event");
        return false;
    }
    true
}
