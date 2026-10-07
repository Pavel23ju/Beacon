# ADR 0001: Start as a modular monolith

- **Status:** Accepted
- **Date:** 2026-10-07

## Context

The target architecture has seven services (incident, triage, dispatch, location, realtime, sync,
gateway) talking over Kafka. Building all of them as separate deployables from day one means
paying for service discovery, network failure handling, distributed tracing and per-service
deployment before any dispatch logic exists. Service boundaries drawn up front are also
often wrong.

## Decision

Run one binary (`cmd/beacon`) with one Go package per future service under `internal/`.
Packages talk through plain Go interfaces and share one Postgres database, but each one owns
its own tables. Every state change is still recorded as an event (see ADR 0002), so the
boundaries already follow the event-driven design.

Split into separate processes in week 4, once the event bus exists and the boundaries have
held up under real code. Dispatch is the first candidate.

## Consequences

- Fast iteration: one `go run`, one debugger, refactors across boundaries are cheap.
- Package boundaries need discipline: `internal/incident` must not query `responders` directly.
- The split in week 4 is mostly wiring (replacing in-process calls with event consumers), not a rewrite.
