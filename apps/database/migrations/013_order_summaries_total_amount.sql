-- projection.order_summaries never had total_amount_minor/currency, but
-- viewservice's projector writes them on every OrderCreated/Confirmed/
-- Cancelled/Expired event — the columns were simply missing.
ALTER TABLE projection.order_summaries
    ADD COLUMN IF NOT EXISTS total_amount_minor BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS currency CHAR(3) NOT NULL DEFAULT 'USD';

ALTER TABLE projection.order_summaries
    ADD CONSTRAINT order_summaries_total_amount_minor_check
    CHECK (total_amount_minor >= 0);
