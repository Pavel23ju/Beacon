-- +goose Up
CREATE EXTENSION IF NOT EXISTS postgis;

CREATE TABLE incidents (
    id              UUID PRIMARY KEY,
    type            TEXT        NOT NULL,
    priority        SMALLINT    NOT NULL DEFAULT 0 CHECK (priority BETWEEN 0 AND 4), -- 0 = untriaged, 1..4 = P1..P4
    status          TEXT        NOT NULL,
    location        GEOGRAPHY(Point, 4326) NOT NULL,
    people_affected INT         NOT NULL DEFAULT 0 CHECK (people_affected >= 0),
    description     TEXT        NOT NULL DEFAULT '',
    reported_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_hlc     BIGINT      NOT NULL
);
CREATE INDEX incidents_status_idx   ON incidents (status);
CREATE INDEX incidents_location_idx ON incidents USING GIST (location);

CREATE TABLE responders (
    id            UUID PRIMARY KEY,
    type          TEXT        NOT NULL,                -- ambulance | fire | rescue
    capabilities  TEXT[]      NOT NULL DEFAULT '{}',
    status        TEXT        NOT NULL DEFAULT 'AVAILABLE',
    last_location GEOGRAPHY(Point, 4326),
    last_seen     TIMESTAMPTZ,
    mode          TEXT        NOT NULL DEFAULT 'online' CHECK (mode IN ('online', 'mesh'))
);
CREATE INDEX responders_location_idx ON responders USING GIST (last_location);

CREATE TABLE assignments (
    id           UUID PRIMARY KEY,
    incident_id  UUID        NOT NULL REFERENCES incidents (id),
    responder_id UUID        NOT NULL REFERENCES responders (id),
    status       TEXT        NOT NULL,
    assigned_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    source       TEXT        NOT NULL DEFAULT 'central' CHECK (source IN ('central', 'mesh'))
);
-- A responder can hold at most one active assignment: the DB-level guard
-- against double-assignment, backing up the locking in the dispatch engine.
CREATE UNIQUE INDEX assignments_one_active_per_responder
    ON assignments (responder_id) WHERE status = 'ACTIVE';

CREATE TABLE events (
    event_id     UUID PRIMARY KEY,
    aggregate_id UUID        NOT NULL,
    type         TEXT        NOT NULL,
    payload      JSONB       NOT NULL,
    hlc          BIGINT      NOT NULL,
    origin_node  TEXT        NOT NULL,
    received_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX events_aggregate_idx ON events (aggregate_id, hlc);

-- +goose Down
DROP TABLE events;
DROP TABLE assignments;
DROP TABLE responders;
DROP TABLE incidents;
