SET search_path TO metrics;

CREATE TABLE business_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id VARCHAR(255) UNIQUE NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    payload JSONB NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE daily_aggregates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    date DATE NOT NULL UNIQUE,
    listing_fill_rate NUMERIC(5,2),
    completion_rate NUMERIC(5,2),
    total_enrollments INTEGER DEFAULT 0,
    total_payouts_cents BIGINT DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_business_events_type ON business_events(event_type);
CREATE INDEX idx_business_events_occurred ON business_events(occurred_at);
