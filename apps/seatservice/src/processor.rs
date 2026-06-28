use crate::db_client::DbClient;
use crate::dtos::{EventEnvelope, OrderCreatedPayload, SeatReservationFailedEvent};
use chrono::Utc;
use rdkafka::producer::{FutureProducer, FutureRecord};
use std::time::Duration;
use tracing::{error, info, warn};
use tracing::Instrument;

pub async fn process_message(db: &DbClient, producer: &FutureProducer, payload_bytes: &[u8], request_id: Option<String>) {
    let req_id_str = request_id.clone().unwrap_or_else(|| "unknown".to_string());
    let span = tracing::info_span!("process_message", request_id = %req_id_str);
    
    async move {
        let envelope: EventEnvelope = match serde_json::from_slice(payload_bytes) {
            Ok(e) => e,
            Err(err) => {
                warn!(error = %err, "Failed to deserialize event envelope");
                return;
            }
        };

    if envelope.event_type == "OrderCreated" {
        let payload_val = match envelope.payload {
            Some(p) => p,
            None => {
                warn!("OrderCreated missing payload");
                return;
            }
        };

        let order: OrderCreatedPayload = match serde_json::from_value(payload_val) {
            Ok(o) => o,
            Err(err) => {
                warn!(error = %err, "Failed to deserialize OrderCreated payload");
                return;
            }
        };

        if order.order_id.is_empty()
            || order.event_id.is_empty()
            || order.section_id.is_empty()
            || order.seat_ids.is_empty()
            || order.reservation_id.is_empty()
        {
            warn!(order_id = %order.order_id, "Missing required fields in OrderCreated");
            return;
        }

        info!(order_id = %order.order_id, "Processing OrderCreated");

        let now = Utc::now();
        match db.process_reservation(&order, now).await {
            Ok(reserved_events) => {
                for event in reserved_events {
                    let msg_str = match serde_json::to_string(&event) {
                        Ok(s) => s,
                        Err(e) => {
                            error!(error = %e, "Failed to serialize SeatReservedEvent");
                            continue;
                        }
                    };

                    let mut record = FutureRecord::to("inventory.events.v1")
                        .payload(&msg_str)
                        .key(&event.aggregate_id);
                    let mut headers = rdkafka::message::OwnedHeaders::new();
                    headers = headers.insert(rdkafka::message::Header {
                        key: "event_type",
                        value: Some("SeatReserved"),
                    });
                    if let Some(ref rid) = request_id {
                        headers = headers.insert(rdkafka::message::Header {
                            key: "X-Request-ID",
                            value: Some(rid),
                        });
                    }
                    record = record.headers(headers);

                    let produce_result = producer.send(record, Duration::from_secs(5)).await;

                    if let Err((e, _)) = produce_result {
                        error!(error = %e, "Failed to produce SeatReservedEvent");
                    }
                }
                info!(order_id = %order.order_id, count = order.seat_ids.len(), "Successfully reserved seats");
            }
            Err(reason) => {
                warn!(order_id = %order.order_id, reason = %reason, "Failed to reserve seats");
                let failed_event = SeatReservationFailedEvent {
                    event_type: "SeatReservationFailed".into(),
                    order_id: order.order_id.clone(),
                    reason,
                };

                if let Ok(msg_str) = serde_json::to_string(&failed_event) {
                    let mut record = FutureRecord::to("inventory.events.v1")
                        .payload(&msg_str)
                        .key(&order.order_id);
                    let mut headers = rdkafka::message::OwnedHeaders::new();
                    headers = headers.insert(rdkafka::message::Header {
                        key: "event_type",
                        value: Some("SeatReservationFailed"),
                    });
                    if let Some(ref rid) = request_id {
                        headers = headers.insert(rdkafka::message::Header {
                            key: "X-Request-ID",
                            value: Some(rid),
                        });
                    }
                    record = record.headers(headers);

                    let _ = producer.send(record, Duration::from_secs(5)).await;
                }
            }
        }
    } else if envelope.event_type == "PaymentFailed" {
        let payload_val = match envelope.payload {
            Some(p) => p,
            None => return,
        };
        let order_id = payload_val.get("order_id").and_then(|v| v.as_str()).unwrap_or("").to_string();
        if order_id.is_empty() { return; }
        
        info!(order_id = %order_id, "Processing PaymentFailed");
        match db.process_payment_failed(&order_id, Utc::now()).await {
            Ok(expired_events) => {
                for event in expired_events {
                    let msg_str = match serde_json::to_string(&event) {
                        Ok(s) => s,
                        Err(_) => continue,
                    };
                    let mut record = FutureRecord::to("inventory.events.v1")
                        .payload(&msg_str)
                        .key(&event.aggregate_id);
                    let mut headers = rdkafka::message::OwnedHeaders::new();
                    headers = headers.insert(rdkafka::message::Header {
                        key: "event_type",
                        value: Some("SeatReservationExpired"),
                    });
                    if let Some(ref rid) = request_id {
                        headers = headers.insert(rdkafka::message::Header {
                            key: "X-Request-ID",
                            value: Some(rid),
                        });
                    }
                    record = record.headers(headers);
                    let _ = producer.send(record, Duration::from_secs(5)).await;
                }
            }
            Err(e) => warn!("Failed to process payment failed: {}", e),
        }
    }
    }
    .instrument(span)
    .await;
}
