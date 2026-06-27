use rdkafka::consumer::{Consumer, StreamConsumer};
use rdkafka::producer::FutureProducer;
use rdkafka::{ClientConfig, Message};
use seatservice::db_client::DbClient;
use seatservice::{logging, processor};
use sqlx::postgres::PgPoolOptions;
use std::env;
use tracing::{error, info, warn};

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    logging::init();

    let db_host = env::var("DATABASE_HOST").unwrap_or_else(|_| "localhost".to_string());
    let db_pass = env::var("POSTGRES_PASSWORD").unwrap_or_else(|_| "velox".to_string());
    let db_url = format!("postgres://velox:{}@{}:5432/velox", db_pass, db_host);

    let pool = PgPoolOptions::new()
        .max_connections(5)
        .connect(&db_url)
        .await?;

    let db_client = DbClient::new(pool.clone());

    let kafka_brokers = env::var("KAFKA_BROKERS").unwrap_or_else(|_| "localhost:9092".to_string());

    let consumer: StreamConsumer = ClientConfig::new()
        .set("group.id", "seatservice_group")
        .set("bootstrap.servers", &kafka_brokers)
        .set("auto.offset.reset", "earliest")
        .set("enable.auto.commit", "true")
        .create()?;

    consumer.subscribe(&["order.events.v1"])?;

    let producer: FutureProducer = ClientConfig::new()
        .set("bootstrap.servers", &kafka_brokers)
        .set("message.timeout.ms", "5000")
        .create()?;

    info!("seatservice started, listening to order.events.v1");

    let (shutdown_tx, mut shutdown_rx) = tokio::sync::watch::channel(false);

    // Healthz endpoint
    tokio::spawn(async move {
        use tokio::io::AsyncWriteExt;
        use tokio::net::TcpListener;

        let listener = TcpListener::bind("0.0.0.0:8080").await.unwrap();
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

    let consumer_task = tokio::spawn(async move {
        loop {
            tokio::select! {
                result = consumer.recv() => {
                    match result {
                        Err(e) => error!(error = %e, "Kafka consumer error"),
                        Ok(m) => {
                            if let Some(payload) = m.payload() {
                                processor::process_message(&db_client, &producer, payload).await;
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
    pool.close().await;

    Ok(())
}
