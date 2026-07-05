use rdkafka::consumer::{CommitMode, Consumer, StreamConsumer};
use rdkafka::producer::FutureProducer;
use rdkafka::ClientConfig;
use seatservice::db_client::DbClient;
use seatservice::processor::MessageMeta;
use seatservice::{expiry, logging, processor};
use sqlx::postgres::PgPoolOptions;
use std::env;
use std::sync::{
    atomic::{AtomicU64, AtomicUsize, Ordering},
    Arc,
};
use std::time::{Duration, SystemTime, UNIX_EPOCH};
use tracing::{error, info};

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    logging::init();

    let db_host = env::var("DATABASE_HOST").unwrap_or_else(|_| "localhost".to_string());
    let db_pass = env::var("DATABASE_PASSWORD").unwrap_or_else(|_| "velox".to_string());
    let db_url = format!("postgres://velox:{}@{}:5432/velox", db_pass, db_host);

    let pool = PgPoolOptions::new()
        .max_connections(5)
        .max_lifetime(Duration::from_secs(30 * 60))
        .idle_timeout(Duration::from_secs(5 * 60))
        .connect(&db_url)
        .await?;

    let db_client = DbClient::new(pool.clone());

    let broker_addrs = env::var("KAFKA_BROKERS").unwrap_or_else(|_| "localhost:9092".to_string());

    let consumer: StreamConsumer = ClientConfig::new()
        .set("group.id", "seatservice_group")
        .set("bootstrap.servers", &broker_addrs)
        .set("auto.offset.reset", "earliest")
        .set("enable.auto.commit", "false")
        .create()?;

    consumer.subscribe(&["order.events.v1"])?;

    let producer: FutureProducer = ClientConfig::new()
        .set("bootstrap.servers", &broker_addrs)
        .set("message.timeout.ms", "5000")
        .create()?;

    info!("seatservice started, listening to order.events.v1");

    let (shutdown_tx, mut shutdown_rx) = tokio::sync::watch::channel(false);

    let broker_errors = Arc::new(AtomicUsize::new(0));
    let broker_first_error = Arc::new(AtomicU64::new(0));

    // Probe endpoint: /healthz is process liveness, /readyz checks dependencies.
    let probe_pool = pool.clone();
    let probe_broker_errors = broker_errors.clone();
    let probe_broker_first_error = broker_first_error.clone();
    tokio::spawn(async move {
        use tokio::io::{AsyncReadExt, AsyncWriteExt};
        use tokio::net::TcpListener;

        let listener = match TcpListener::bind("0.0.0.0:8080").await {
            Ok(l) => l,
            Err(e) => {
                tracing::error!("Failed to bind healthz server: {}", e);
                return;
            }
        };
        loop {
            tokio::select! {
                Ok((mut socket, _)) = listener.accept() => {
                    let mut buf = [0_u8; 512];
                    let n = socket.read(&mut buf).await.unwrap_or(0);
                    let request = String::from_utf8_lossy(&buf[..n]);
                    let path = request
                        .lines()
                        .next()
                        .and_then(|line| line.split_whitespace().nth(1))
                        .unwrap_or("/healthz");
                    let ready = if path == "/readyz" {
                        let db_ready = tokio::time::timeout(
                            Duration::from_secs(2),
                            sqlx::query("SELECT 1").execute(&probe_pool),
                        )
                        .await
                        .is_ok_and(|result| result.is_ok());
                        db_ready && !broker_degraded(
                            &probe_broker_errors,
                            &probe_broker_first_error,
                        )
                    } else {
                        true
                    };
                    let response = if ready {
                        "HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nOK"
                    } else {
                        "HTTP/1.1 503 Service Unavailable\r\nContent-Length: 8\r\n\r\nDEGRADED"
                    };
                    let _ = socket.write_all(response.as_bytes()).await;
                }
                _ = shutdown_rx.changed() => {
                    info!("healthz server shutting down");
                    break;
                }
            }
        }
    });

    let mut shutdown_rx2 = shutdown_tx.subscribe();
    let consumer_broker_errors = broker_errors.clone();
    let consumer_broker_first_error = broker_first_error.clone();

    let expiry_task = tokio::spawn(expiry::run(
        db_client.clone(),
        producer.clone(),
        shutdown_tx.subscribe(),
    ));

    let consumer_task = tokio::spawn(async move {
        loop {
            tokio::select! {
                result = consumer.recv() => {
                    match result {
                        Err(e) => {
                            note_broker_error(&consumer_broker_errors, &consumer_broker_first_error);
                            error!(error = %e, "Broker consumer error");
                        }
                        Ok(m) => {
                            use rdkafka::message::Message;
                            let mut req_id = None;
                            if let Some(headers) = m.headers() {
                                use rdkafka::message::Headers;
                                for i in 0..headers.count() {
                                    let h = headers.get(i);
                                    if h.key == "X-Request-ID" {
                                            if let Some(v) = h.value {
                                                req_id = Some(String::from_utf8_lossy(v).into_owned());
                                            }
                                        }
                                    }
                                }
                            if let Some(payload) = m.payload() {
                                let meta = MessageMeta {
                                    source_partition: m.partition(),
                                    source_offset: m.offset(),
                                    request_id: req_id,
                                };
                                let should_commit = processor::process_message(&db_client, &producer, payload, meta).await;
                                if should_commit {
                                    note_broker_success(&consumer_broker_errors, &consumer_broker_first_error);
                                    if let Err(e) = consumer.commit_message(&m, CommitMode::Async) {
                                        error!(error = %e, "Failed to commit offset");
                                    }
                                } else {
                                    note_broker_error(&consumer_broker_errors, &consumer_broker_first_error);
                                }
                            }
                        }
                    }
                }
                _ = shutdown_rx2.changed() => {
                    info!("consumer shutting down");
                    break;
                }
            }
        }
    });

    tokio::signal::ctrl_c()
        .await
        .expect("Failed to install Ctrl+C signal handler");

    info!("shutting down");
    let _ = shutdown_tx.send(true);
    let _ = consumer_task.await;
    let _ = expiry_task.await;
    pool.close().await;

    Ok(())
}

fn note_broker_error(errors: &AtomicUsize, first_error: &AtomicU64) {
    if errors.fetch_add(1, Ordering::Relaxed) == 0 {
        first_error.store(unix_secs_now(), Ordering::Relaxed);
    }
}

fn note_broker_success(errors: &AtomicUsize, first_error: &AtomicU64) {
    errors.store(0, Ordering::Relaxed);
    first_error.store(0, Ordering::Relaxed);
}

fn broker_degraded(errors: &AtomicUsize, first_error: &AtomicU64) -> bool {
    if errors.load(Ordering::Relaxed) >= 5 {
        return true;
    }
    let first_error_at = first_error.load(Ordering::Relaxed);
    first_error_at != 0 && unix_secs_now().saturating_sub(first_error_at) >= 30
}

fn unix_secs_now() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_secs()
}
