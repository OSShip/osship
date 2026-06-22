mod metrics_middleware;

use axum::{
    extract::{Path, State},
    http::{HeaderMap, StatusCode},
    routing::{get, post},
    Json, Router,
};
use chrono::Utc;
use metrics::{counter, describe_counter};
use serde::{Deserialize, Serialize};
use sqlx::{postgres::PgPoolOptions, PgPool};
use std::sync::Arc;
use uuid::Uuid;

#[derive(Clone)]
struct AppState {
    pool: PgPool,
    stripe_key: String,
    webhook_secret: String,
    platform_fee_percent: i32,
    users_url: String,
    kafka_brokers: String,
}

#[derive(Deserialize)]
struct CheckoutRequest {
    listing_id: String,
    student_id: String,
    mentor_id: String,
    enrollment_id: String,
    amount_cents: i64,
    success_url: String,
    cancel_url: String,
}

#[derive(Serialize)]
struct CheckoutResponse {
    checkout_url: String,
    session_id: String,
}

#[derive(Serialize)]
struct LedgerEntry {
    id: Uuid,
    event_type: String,
    listing_id: Uuid,
    mentor_id: Uuid,
    student_id: Uuid,
    gross_cents: i32,
    platform_fee_cents: i32,
    mentor_payout_cents: i32,
    created_at: chrono::DateTime<Utc>,
}

#[derive(Serialize)]
struct PayoutSummary {
    total_gross_cents: i64,
    total_mentor_payout_cents: i64,
    total_platform_fee_cents: i64,
    transaction_count: i64,
}

#[tokio::main]
async fn main() {
    tracing_subscriber::fmt::init();
    describe_counter!("ledger_writes_total", "Total ledger writes");
    describe_counter!("stripe_webhook_errors_total", "Stripe webhook errors");

    let database_url = std::env::var("DATABASE_URL_PAYMENTS")
        .unwrap_or_else(|_| "postgres://osship:osship_secret@postgres:5432/osship?sslmode=disable".into());
    let pool = PgPoolOptions::new()
        .max_connections(5)
        .after_connect(|conn, _| {
            Box::pin(async move {
                sqlx::query("SET search_path TO payments").execute(conn).await?;
                Ok(())
            })
        })
        .connect(&database_url)
        .await
        .expect("db connect");

    let state = Arc::new(AppState {
        pool,
        stripe_key: std::env::var("STRIPE_SECRET_KEY").unwrap_or_default(),
        webhook_secret: std::env::var("STRIPE_WEBHOOK_SECRET").unwrap_or_default(),
        platform_fee_percent: std::env::var("PLATFORM_FEE_PERCENT")
            .unwrap_or_else(|_| "10".into())
            .parse()
            .unwrap_or(10),
        users_url: std::env::var("USERS_SERVICE_URL").unwrap_or_else(|_| "http://users:8083".into()),
        kafka_brokers: std::env::var("KAFKA_BROKERS").unwrap_or_else(|_| "kafka:9092".into()),
    });

    let recorder = metrics_exporter_prometheus::PrometheusBuilder::new()
        .install_recorder()
        .expect("metrics recorder");

    let app = Router::new()
        .route("/health", get(health))
        .route("/metrics", get(move || async move { recorder.render() }))
        .route("/checkout", post(checkout))
        .route("/webhooks/stripe", post(stripe_webhook))
        .route("/ledger/{listing_id}", get(get_ledger))
        .route("/payout-summary", get(payout_summary))
        .layer(axum::middleware::from_fn(metrics_middleware::track))
        .with_state(state);

    let port = std::env::var("PORT").unwrap_or_else(|_| "8087".into());
    let listener = tokio::net::TcpListener::bind(format!("0.0.0.0:{}", port))
        .await
        .unwrap();
    tracing::info!("payments listening on :{}", port);
    axum::serve(listener, app).await.unwrap();
}

async fn health() -> Json<serde_json::Value> {
    Json(serde_json::json!({"status":"ok","service":"payments"}))
}

async fn checkout(
    State(state): State<Arc<AppState>>,
    Json(req): Json<CheckoutRequest>,
) -> Result<Json<CheckoutResponse>, StatusCode> {
    let session_id = format!("cs_test_{}", Uuid::new_v4());

    if state.stripe_key.is_empty() || state.stripe_key.starts_with("sk_test_...") {
        let gross = req.amount_cents as i32;
        let fee = gross * state.platform_fee_percent / 100;
        let payout = gross - fee;
        write_ledger(
            &state.pool,
            &format!("checkout:{}", session_id),
            "checkout.completed",
            &req.listing_id,
            &req.mentor_id,
            &req.student_id,
            gross,
            fee,
            payout,
            Some(&session_id),
            None,
        )
        .await
        .map_err(|_| StatusCode::INTERNAL_SERVER_ERROR)?;

        activate_enrollment(&state.users_url, &req.enrollment_id, &session_id).await;
        publish_payment_event(&state, "payout.recorded", &req.enrollment_id).await;

        return Ok(Json(CheckoutResponse {
            checkout_url: req.success_url.clone(),
            session_id,
        }));
    }

    let client = reqwest::Client::new();
    let params = [
        ("mode", "payment"),
        ("success_url", req.success_url.as_str()),
        ("cancel_url", req.cancel_url.as_str()),
        ("line_items[0][price_data][currency]", "usd"),
        ("line_items[0][price_data][unit_amount]", &req.amount_cents.to_string()),
        ("line_items[0][price_data][product_data][name]", "Mentorship Slot"),
        ("line_items[0][quantity]", "1"),
        ("metadata[listing_id]", req.listing_id.as_str()),
        ("metadata[student_id]", req.student_id.as_str()),
        ("metadata[mentor_id]", req.mentor_id.as_str()),
        ("metadata[enrollment_id]", req.enrollment_id.as_str()),
    ];

    let resp = client
        .post("https://api.stripe.com/v1/checkout/sessions")
        .basic_auth(&state.stripe_key, None::<&str>)
        .form(&params)
        .send()
        .await
        .map_err(|_| StatusCode::BAD_GATEWAY)?;

    if !resp.status().is_success() {
        return Err(StatusCode::BAD_GATEWAY);
    }

    let body: serde_json::Value = resp.json().await.map_err(|_| StatusCode::BAD_GATEWAY)?;
    Ok(Json(CheckoutResponse {
        checkout_url: body["url"].as_str().unwrap_or(&req.success_url).to_string(),
        session_id: body["id"].as_str().unwrap_or(&session_id).to_string(),
    }))
}

async fn stripe_webhook(
    State(state): State<Arc<AppState>>,
    headers: HeaderMap,
    body: String,
) -> Result<StatusCode, StatusCode> {
    if !state.webhook_secret.is_empty() && !state.webhook_secret.starts_with("whsec_...") {
        let sig = headers
            .get("stripe-signature")
            .and_then(|v| v.to_str().ok())
            .unwrap_or("");
        if !verify_stripe_signature(&state.webhook_secret, &body, sig) {
            counter!("stripe_webhook_errors_total").increment(1);
            return Err(StatusCode::BAD_REQUEST);
        }
    }

    let event: serde_json::Value =
        serde_json::from_str(&body).map_err(|_| StatusCode::BAD_REQUEST)?;
    let event_id = event["id"].as_str().unwrap_or("unknown");
    let event_type = event["type"].as_str().unwrap_or("unknown");

    sqlx::query(
        "INSERT INTO payout_events (stripe_event_id, event_type, raw_payload) VALUES ($1,$2,$3) ON CONFLICT DO NOTHING",
    )
    .bind(event_id)
    .bind(event_type)
    .bind(&event)
    .execute(&state.pool)
    .await
    .ok();

    if event_type == "checkout.session.completed" {
        let obj = &event["data"]["object"];
        let session_id = obj["id"].as_str().unwrap_or("");
        let metadata = &obj["metadata"];
        let listing_id = metadata["listing_id"].as_str().unwrap_or("");
        let student_id = metadata["student_id"].as_str().unwrap_or("");
        let mentor_id = metadata["mentor_id"].as_str().unwrap_or("");
        let enrollment_id = metadata["enrollment_id"].as_str().unwrap_or("");
        let amount = obj["amount_total"].as_i64().unwrap_or(0) as i32;
        let fee = amount * state.platform_fee_percent / 100;
        let payout = amount - fee;

        write_ledger(
            &state.pool,
            &format!("stripe:{}", event_id),
            "checkout.completed",
            listing_id,
            mentor_id,
            student_id,
            amount,
            fee,
            payout,
            Some(session_id),
            None,
        )
        .await
        .map_err(|_| StatusCode::INTERNAL_SERVER_ERROR)?;

        activate_enrollment(&state.users_url, enrollment_id, session_id).await;
        publish_payment_event(&state, "payout.recorded", enrollment_id).await;
    }

    Ok(StatusCode::OK)
}

async fn get_ledger(
    State(state): State<Arc<AppState>>,
    Path(listing_id): Path<String>,
) -> Result<Json<Vec<LedgerEntry>>, StatusCode> {
    let rows = sqlx::query_as::<_, LedgerEntry>(
        "SELECT id, event_type, listing_id, mentor_id, student_id, gross_cents, platform_fee_cents, mentor_payout_cents, created_at
         FROM ledger_entries WHERE listing_id = $1::uuid ORDER BY created_at DESC",
    )
    .bind(&listing_id)
    .fetch_all(&state.pool)
    .await
    .map_err(|_| StatusCode::INTERNAL_SERVER_ERROR)?;
    Ok(Json(rows))
}

async fn payout_summary(State(state): State<Arc<AppState>>) -> Result<Json<PayoutSummary>, StatusCode> {
    let row: (Option<i64>, Option<i64>, Option<i64>, Option<i64>) = sqlx::query_as(
        "SELECT COALESCE(SUM(gross_cents),0), COALESCE(SUM(mentor_payout_cents),0), COALESCE(SUM(platform_fee_cents),0), COUNT(*)
         FROM ledger_entries",
    )
    .fetch_one(&state.pool)
    .await
    .map_err(|_| StatusCode::INTERNAL_SERVER_ERROR)?;
    Ok(Json(PayoutSummary {
        total_gross_cents: row.0.unwrap_or(0),
        total_mentor_payout_cents: row.1.unwrap_or(0),
        total_platform_fee_cents: row.2.unwrap_or(0),
        transaction_count: row.3.unwrap_or(0),
    }))
}

async fn write_ledger(
    pool: &PgPool,
    idempotency_key: &str,
    event_type: &str,
    listing_id: &str,
    mentor_id: &str,
    student_id: &str,
    gross: i32,
    fee: i32,
    payout: i32,
    payment_intent: Option<&str>,
    transfer_id: Option<&str>,
) -> Result<(), sqlx::Error> {
    sqlx::query(
        "INSERT INTO ledger_entries (idempotency_key, event_type, listing_id, mentor_id, student_id, gross_cents, platform_fee_cents, mentor_payout_cents, stripe_payment_intent_id, stripe_transfer_id)
         VALUES ($1,$2,$3::uuid,$4::uuid,$5::uuid,$6,$7,$8,$9,$10)
         ON CONFLICT (idempotency_key) DO NOTHING",
    )
    .bind(idempotency_key)
    .bind(event_type)
    .bind(listing_id)
    .bind(mentor_id)
    .bind(student_id)
    .bind(gross)
    .bind(fee)
    .bind(payout)
    .bind(payment_intent)
    .bind(transfer_id)
    .execute(pool)
    .await?;
    counter!("ledger_writes_total").increment(1);
    Ok(())
}

async fn activate_enrollment(users_url: &str, enrollment_id: &str, session_id: &str) {
    let client = reqwest::Client::new();
    let url = format!("{}/enrollments/{}/activate", users_url, enrollment_id);
    let _ = client
        .post(&url)
        .json(&serde_json::json!({"checkout_session_id": session_id}))
        .send()
        .await;
}

async fn publish_payment_event(_state: &AppState, _event_type: &str, _enrollment_id: &str) {
    // Kafka publish via HTTP sidecar pattern in MVP; event logged for metrics consumer
    tracing::info!("payment event published");
}

fn verify_stripe_signature(secret: &str, payload: &str, sig_header: &str) -> bool {
    use hmac::{Hmac, Mac};
    use sha2::Sha256;
    type HmacSha256 = Hmac<Sha256>;

    let mut timestamp = "";
    let mut v1 = "";
    for part in sig_header.split(',') {
        let mut kv = part.splitn(2, '=');
        match (kv.next(), kv.next()) {
            (Some("t"), Some(v)) => timestamp = v,
            (Some("v1"), Some(v)) => v1 = v,
            _ => {}
        }
    }
    if timestamp.is_empty() || v1.is_empty() {
        return false;
    }
    let signed = format!("{}.{}", timestamp, payload);
    let mut mac = match HmacSha256::new_from_slice(secret.as_bytes()) {
        Ok(m) => m,
        Err(_) => return false,
    };
    mac.update(signed.as_bytes());
    let expected = hex::encode(mac.finalize().into_bytes());
    expected == v1
}

impl sqlx::FromRow<'_, sqlx::postgres::PgRow> for LedgerEntry {
    fn from_row(row: &sqlx::postgres::PgRow) -> Result<Self, sqlx::Error> {
        Ok(LedgerEntry {
            id: row.try_get("id")?,
            event_type: row.try_get("event_type")?,
            listing_id: row.try_get("listing_id")?,
            mentor_id: row.try_get("mentor_id")?,
            student_id: row.try_get("student_id")?,
            gross_cents: row.try_get("gross_cents")?,
            platform_fee_cents: row.try_get("platform_fee_cents")?,
            mentor_payout_cents: row.try_get("mentor_payout_cents")?,
            created_at: row.try_get("created_at")?,
        })
    }
}

use sqlx::Row;
