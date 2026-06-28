BEGIN;

-- Add Venues
INSERT INTO catalog.venues (id, name, city, address, capacity) VALUES
('ven_velox_arena', 'Velox Arena', 'Chicago', '100 Arena Way', 10000),
('ven_north_pier', 'North Pier Hall', 'Seattle', '200 Pier St', 5000),
('ven_civic_bowl', 'Civic Bowl', 'Denver', '300 Bowl Ave', 15000),
('ven_moonlight', 'Moonlight Grounds', 'Austin', '400 Moon Rd', 20000);

-- Add Events
-- 'evt_neon_riot', 'evt_north_pier', 'evt_civic_bowl', 'evt_summer_fests'
INSERT INTO catalog.events (id, venue_id, name, starts_at, status) VALUES
('evt_neon_riot', 'ven_velox_arena', 'Neon Riot Live', '2026-08-15 20:00:00+00', 'AVAILABLE'),
('evt_north_pier', 'ven_north_pier', 'North Pier Symphony', '2026-09-10 19:30:00+00', 'AVAILABLE'),
('evt_civic_bowl', 'ven_civic_bowl', 'Civic Bowl Championship', '2026-10-05 18:00:00+00', 'AVAILABLE'),
('evt_summer_fests', 'ven_moonlight', 'Summer Solstice Festival', '2026-07-20 12:00:00+00', 'AVAILABLE');

-- Also add Midnight Array and other events just to match the old mock data, even though they might not have reservations?
-- Actually, the frontend image SVGs are "event-midnight-array.svg", etc.
-- And the frontend `client.ts` maps `event-midnight-array.svg` hardcoded. So the name can be whatever.
-- But wait! The UI had `<p>Midnight Array</p>`. That's because the title in `mock.ts` was Midnight Array.
-- But the frontend auditor deleted `mock.ts` and now uses real events!
-- So the name should be Neon Riot Live, North Pier Symphony, etc.

COMMIT;
