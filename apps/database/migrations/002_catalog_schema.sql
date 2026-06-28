CREATE SCHEMA IF NOT EXISTS catalog;

CREATE TABLE IF NOT EXISTS catalog.users (
    id UUID PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS catalog.venues (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    city TEXT NOT NULL,
    address TEXT NOT NULL,
    capacity INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS catalog.venue_seats (
    venue_id UUID REFERENCES catalog.venues(id),
    section_id TEXT NOT NULL,
    seat_id TEXT NOT NULL,
    PRIMARY KEY (venue_id, section_id, seat_id)
);

CREATE TABLE IF NOT EXISTS catalog.user_venues (
    user_id UUID REFERENCES catalog.users(id),
    venue_id UUID REFERENCES catalog.venues(id),
    venue_role TEXT NOT NULL,
    PRIMARY KEY (user_id, venue_id)
);

CREATE TABLE IF NOT EXISTS catalog.events (
    id UUID PRIMARY KEY,
    venue_id UUID REFERENCES catalog.venues(id),
    name TEXT NOT NULL,
    starts_at TIMESTAMP WITH TIME ZONE NOT NULL,
    status TEXT NOT NULL
);
