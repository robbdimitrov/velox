use rdkafka::consumer::{CommitMode, Consumer, StreamConsumer};
use rdkafka::producer::FutureProducer;
use rdkafka::ClientConfig;
use seatservice::db_client::DbClient;
use seatservice::processor::MessageMeta;
use seatservice::{expiry, logging, processor};
use sqlx::postgres::PgPoolOptions;
use std::env;
use tracing::{error, info};

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    logging::init();

    let db_host = env::var("DATABASE_HOST").unwrap_or_else(|_| "localhost".to_string());
    let db_pass = env::var("DATABASE_PASSWORD").unwrap_or_else(|_| "velox".to_string());
    let db_url = format!("postgres://velox:{}@{}:5432/velox", db_pass, db_host);

    let pool = PgPoolOptions::new()
        .max_connections(5)
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

    // Healthz endpoint
    tokio::spawn(async move {
        use tokio::io::AsyncWriteExt;
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
                    let response = "HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nOK";
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
                        Err(e) => error!(error = %e, "Broker consumer error"),
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
                                    if let Err(e) = consumer.commit_message(&m, CommitMode::Async) {
                                        error!(error = %e, "Failed to commit offset");
                                    }
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
