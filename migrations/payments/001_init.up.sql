SET search_path TO payments;

CREATE TABLE ledger_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    idempotency_key VARCHAR(255) UNIQUE NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    listing_id UUID NOT NULL,
    mentor_id UUID NOT NULL,
    student_id UUID NOT NULL,
    gross_cents INTEGER NOT NULL,
    platform_fee_cents INTEGER NOT NULL,
    mentor_payout_cents INTEGER NOT NULL,
    stripe_payment_intent_id VARCHAR(255),
    stripe_transfer_id VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE payout_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    stripe_event_id VARCHAR(255) UNIQUE NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    raw_payload JSONB NOT NULL,
    processed BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE OR REPLACE FUNCTION prevent_ledger_mutation()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'ledger_entries is append-only: % not allowed', TG_OP;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER ledger_no_update
    BEFORE UPDATE OR DELETE ON ledger_entries
    FOR EACH ROW EXECUTE FUNCTION prevent_ledger_mutation();

CREATE INDEX idx_ledger_listing ON ledger_entries(listing_id);
CREATE INDEX idx_ledger_mentor ON ledger_entries(mentor_id);
CREATE INDEX idx_ledger_created ON ledger_entries(created_at);
