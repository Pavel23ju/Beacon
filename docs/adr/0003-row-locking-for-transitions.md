# ADR 0003: Serialize state changes with row locks

- **Status:** Accepted
- **Date:** 2026-10-07

## Context

Several actors can try to change the same incident at once: the responder acknowledging, an
acknowledgement-timeout worker reassigning, a dispatcher preempting. If each reads the
status, validates and then writes, two of them can both pass validation (a lost update or an
illegal double transition).

## Decision

`incident.Store.Transition` runs `SELECT status … FOR UPDATE` on the incident row, validates
against the state machine, then updates and appends the event, all in one transaction.
Concurrent transitions on the same incident wait in line, and each one is validated against
the real current state.

As a safety net for week 3, a partial unique index
(`assignments_one_active_per_responder`) makes it impossible at the database level for a
responder to hold two active assignments.

## Evidence

`TestConcurrentTransitionsSerialize` sends 50 parallel `DISPATCHED → ACKNOWLEDGED` requests.
Exactly 1 succeeds and 49 get HTTP 409.

## Alternatives considered

- **Optimistic concurrency** (`UPDATE … WHERE status = $from`, retry on 0 rows): no lock wait,
  but under heavy contention the retries are wasted work. A good option if contention turns out low.
- **Redis distributed lock:** adds a second system to the correctness path and has
  lease-expiry edge cases. Kept as an option for dispatch across processes in week 3.
