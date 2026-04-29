//! Mock edge gateway: deterministic PLC-style telemetry into NATS JetStream (see planning Phase 1).

use async_nats::jetstream;
use serde::Serialize;
use std::time::{Duration, SystemTime, UNIX_EPOCH};
use tokio::time::interval;

#[derive(Serialize)]
struct PlcSample {
    station_id: String,
    torque: f64,
    timestamp: i64,
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
    let url = std::env::var("NATS_URL").unwrap_or_else(|_| "nats://127.0.0.1:4222".into());
    let station = std::env::var("EDGE_STATION_ID").unwrap_or_else(|_| "5".into());
    let hz: u64 = std::env::var("EDGE_HZ")
        .ok()
        .and_then(|s| s.parse().ok())
        .unwrap_or(20);

    let client = async_nats::connect(url).await?;
    let js = jetstream::new(client);

    let mut tick = interval(Duration::from_millis(1000 / hz.max(1)));
    let mut phase: f64 = 0.0;

    println!("Aegis edge gateway: station={} ~{}Hz -> aegis.telemetry.raw", station, hz);

    let publish_loop = async {
        loop {
            tick.tick().await;
            phase += 0.15;
            let torque = 40.0 + 5.0 * phase.sin() + (phase * 7.0).sin() * 0.5;
            let ts = SystemTime::now()
                .duration_since(UNIX_EPOCH)
                .map(|d| d.as_millis() as i64)
                .unwrap_or(0);

            let sample = PlcSample {
                station_id: station.clone(),
                torque,
                timestamp: ts,
            };
            let payload = serde_json::to_vec(&sample)?;
            js.publish("aegis.telemetry.raw".to_string(), payload.into())
                .await?;
        }
        #[allow(unreachable_code)]
        Ok::<(), Box<dyn std::error::Error + Send + Sync>>(())
    };

    tokio::select! {
        result = publish_loop => result?,
        _ = tokio::signal::ctrl_c() => {
            println!("Aegis edge gateway: shutting down");
        }
    }

    Ok(())
}
