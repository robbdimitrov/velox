use crate::db_client::DbClient;
use crate::processor::publish;
use chrono::Utc;
use rdkafka::producer::FutureProducer;
use tracing::{error, info};

const SWEEP_INTERVAL: std::time::Duration = std::time::Duration::from_secs(5);

/// Periodically expires reservations whose hold deadline has passed with no
/// follow-up payment outcome (success or failure). Without this sweep a held
/// seat with no confirming/failing order event would stay HELD forever.
pub async fn run(
    db: DbClient,
    producer: FutureProducer,
    mut shutdown_rx: tokio::sync::watch::Receiver<bool>,
) {
    let mut ticker = tokio::time::interval(SWEEP_INTERVAL);
    loop {
        tokio::select! {
            _ = ticker.tick() => {
                match db.expire_due_reservations(Utc::now()).await {
                    Ok(events) => {
                        if !events.is_empty() {
                            info!(count = events.len(), "Expired due reservations");
                        }
                        for event in events {
                            publish(&producer, &event.event_type.clone(), &event.aggregate_id.clone(), &event, None).await;
                        }
                    }
                    Err(e) => error!(error = %e, "Failed to sweep expired reservations"),
                }
            }
            _ = shutdown_rx.changed() => {
                info!("expiry scheduler shutting down");
                break;
            }
        }
    }
}
