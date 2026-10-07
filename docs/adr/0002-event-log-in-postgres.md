# ADR 0002: Event log in Postgres, written in the same transaction

- **Status:** Accepted
- **Date:** 2026-10-07

## Context

Every incident and responder change must be in a replayable audit trail. It feeds the control
room, the eventual Kafka bus, and the offline sync in week 8. Writing the state row to
Postgres and then publishing to Kafka separately (a "dual write") can lose or invent events
when one of the two fails.

## Decision

Keep both a current-state table (`incidents`, `responders`) and an append-only `events`
table. Each change updates the state row **and** appends its event in **one Postgres
transaction** (`internal/eventstore.Append`).

- Each event has `event_id` (UUID, for idempotency), `aggregate_id`, `type`, a JSONB
  `payload`, an `hlc` timestamp (hybrid logical clock, `pkg/hlc`) and `origin_node`.
- `incident.Replay` rebuilds an incident from its events and re-validates every transition.
  Integration tests assert that the replayed state equals the stored state.
- Kafka/Redpanda arrives in week 4 via the **transactional outbox** pattern: a relay publishes
  rows from `events` that haven't been published yet. Postgres stays the source of truth.

## Alternatives considered

- **Pure event sourcing, no state tables:** every read replays events. More elegant, but
  dispatch queries ("available responders near X") need indexed state. Rejected for now.
- **Kafka as the source of truth:** needs Kafka from day one and makes "read your own write"
  harder. Rejected.

## Consequences

- Events and state can't diverge: they commit or roll back together.
- Rejected transitions are never logged.
- HLC timestamps from day one mean backend events and mesh-originated events (week 8) can be
  merged into one ordered timeline.
