BEGIN;

ALTER TABLE catalog.events
    ADD COLUMN IF NOT EXISTS category TEXT NOT NULL DEFAULT 'Concerts',
    ADD COLUMN IF NOT EXISTS description TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS timezone TEXT NOT NULL DEFAULT 'UTC',
    ADD COLUMN IF NOT EXISTS status_reason TEXT,
    ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

ALTER TABLE catalog.events
    ADD CONSTRAINT events_category_check
    CHECK (category IN ('Concerts', 'Sports', 'Theatre', 'Festivals'));

CREATE TABLE IF NOT EXISTS catalog.venue_sections (
    venue_id TEXT NOT NULL REFERENCES catalog.venues(id),
    section_id TEXT NOT NULL,
    name TEXT NOT NULL,
    display_order INTEGER NOT NULL DEFAULT 0,
    width INTEGER NOT NULL CHECK (width > 0),
    height INTEGER NOT NULL CHECK (height > 0),
    PRIMARY KEY (venue_id, section_id)
);

ALTER TABLE catalog.venue_seats
    ADD COLUMN IF NOT EXISTS row_label TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS seat_number INTEGER NOT NULL DEFAULT 0 CHECK (seat_number >= 0),
    ADD COLUMN IF NOT EXISTS x INTEGER NOT NULL DEFAULT 0 CHECK (x >= 0),
    ADD COLUMN IF NOT EXISTS y INTEGER NOT NULL DEFAULT 0 CHECK (y >= 0),
    ADD COLUMN IF NOT EXISTS accessibility BOOLEAN NOT NULL DEFAULT false;

INSERT INTO catalog.venue_sections (
    venue_id,
    section_id,
    name,
    display_order,
    width,
    height
)
SELECT v.id, section_id, section_id || ' Section', display_order, 464, 204
FROM catalog.venues v
CROSS JOIN (VALUES ('A', 1), ('B', 2)) AS sections(section_id, display_order)
ON CONFLICT (venue_id, section_id) DO NOTHING;

WITH generated_seats AS (
    SELECT
        v.id AS venue_id,
        sections.section_id,
        row_label,
        seat_number,
        row_label || '-' || lpad(seat_number::text, 2, '0') AS seat_id,
        44 + (seat_number - 1) * 42 AS x,
        42 + (ascii(row_label) - ascii('A')) * 42 AS y,
        seat_number IN (1, 10) AS accessibility
    FROM catalog.venues v
    CROSS JOIN (VALUES ('A'), ('B')) AS sections(section_id)
    CROSS JOIN unnest(ARRAY['A', 'B', 'C', 'D']) AS row_label
    CROSS JOIN generate_series(1, 10) AS seat_number
)
INSERT INTO catalog.venue_seats (
    venue_id,
    section_id,
    seat_id,
    row_label,
    seat_number,
    x,
    y,
    accessibility
)
SELECT venue_id, section_id, seat_id, row_label, seat_number, x, y, accessibility
FROM generated_seats
ON CONFLICT (venue_id, section_id, seat_id) DO NOTHING;

WITH normalized AS (
    SELECT
        venue_id,
        section_id,
        seat_id,
        substring(seat_id FROM '^[^-]+') AS row_label,
        COALESCE(substring(seat_id FROM '-([0-9]+)$')::INTEGER, 0) AS seat_number
    FROM catalog.venue_seats
)
UPDATE catalog.venue_seats vs
SET row_label = normalized.row_label,
    seat_number = normalized.seat_number,
    x = CASE WHEN normalized.seat_number > 0 THEN 44 + (normalized.seat_number - 1) * 42 ELSE 0 END,
    y = CASE WHEN normalized.row_label <> '' THEN 42 + (ascii(normalized.row_label) - ascii('A')) * 42 ELSE 0 END,
    accessibility = normalized.seat_number IN (1, 10)
FROM normalized
WHERE vs.venue_id = normalized.venue_id
  AND vs.section_id = normalized.section_id
  AND vs.seat_id = normalized.seat_id;

INSERT INTO catalog.venue_sections (
    venue_id,
    section_id,
    name,
    display_order,
    width,
    height
)
SELECT
    venue_id,
    section_id,
    section_id || ' Section',
    dense_rank() OVER (PARTITION BY venue_id ORDER BY section_id),
    464,
    204
FROM (
    SELECT DISTINCT venue_id, section_id
    FROM catalog.venue_seats
) sections
ON CONFLICT (venue_id, section_id) DO NOTHING;

CREATE TABLE IF NOT EXISTS catalog.event_sections (
    event_id TEXT NOT NULL REFERENCES catalog.events(id) ON DELETE CASCADE,
    section_id TEXT NOT NULL,
    name TEXT NOT NULL,
    display_order INTEGER NOT NULL DEFAULT 0,
    width INTEGER NOT NULL CHECK (width > 0),
    height INTEGER NOT NULL CHECK (height > 0),
    PRIMARY KEY (event_id, section_id)
);

INSERT INTO catalog.event_sections (
    event_id,
    section_id,
    name,
    display_order,
    width,
    height
)
SELECT
    e.id,
    vs.section_id,
    vs.name,
    vs.display_order,
    vs.width,
    vs.height
FROM catalog.events e
JOIN catalog.venue_sections vs ON vs.venue_id = e.venue_id
ON CONFLICT (event_id, section_id) DO UPDATE
SET name = EXCLUDED.name,
    display_order = EXCLUDED.display_order,
    width = EXCLUDED.width,
    height = EXCLUDED.height;

ALTER TABLE projection.seat_snapshots
    ADD COLUMN IF NOT EXISTS row_label TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS seat_number INTEGER NOT NULL DEFAULT 0 CHECK (seat_number >= 0),
    ADD COLUMN IF NOT EXISTS x INTEGER NOT NULL DEFAULT 0 CHECK (x >= 0),
    ADD COLUMN IF NOT EXISTS y INTEGER NOT NULL DEFAULT 0 CHECK (y >= 0),
    ADD COLUMN IF NOT EXISTS accessibility BOOLEAN NOT NULL DEFAULT false;

WITH normalized AS (
    SELECT
        event_id,
        section_id,
        seat_id,
        substring(seat_id FROM '^[^-]+') AS row_label,
        COALESCE(substring(seat_id FROM '-([0-9]+)$')::INTEGER, 0) AS seat_number
    FROM projection.seat_snapshots
)
UPDATE projection.seat_snapshots ss
SET row_label = normalized.row_label,
    seat_number = normalized.seat_number,
    x = CASE WHEN normalized.seat_number > 0 THEN 44 + (normalized.seat_number - 1) * 42 ELSE 0 END,
    y = CASE WHEN normalized.row_label <> '' THEN 42 + (ascii(normalized.row_label) - ascii('A')) * 42 ELSE 0 END,
    accessibility = normalized.seat_number IN (1, 10)
FROM normalized
WHERE ss.event_id = normalized.event_id
  AND ss.section_id = normalized.section_id
  AND ss.seat_id = normalized.seat_id;

COMMIT;
