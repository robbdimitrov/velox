use crate::db_client::DbClient;
use crate::dtos::{EventEnvelope, OrderCreatedPayload, SeatReservationFailedEvent};
use chrono::Utc;
use rdkafka::producer::{FutureProducer, FutureRecord};
use std::time::Duration;
use tracing::{error, info, warn};

pub async fn process_message(db: &DbClient, producer: &FutureProducer, payload_bytes: &[u8]) {
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

                    let produce_result = producer
                        .send(
                            FutureRecord::to("inventory.events.v1")
                                .payload(&msg_str)
                                .key(&event.aggregate_id),
                            Duration::from_secs(5),
                        )
                        .await;

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
                    let _ = producer
                        .send(
                            FutureRecord::to("inventory.events.v1")
                                .payload(&msg_str)
                                .key(&order.order_id),
                            Duration::from_secs(5),
                        )
                        .await;
                }
            }
        }
    }
}
