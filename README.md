# Beacon

**Resilient emergency dispatch that keeps working when the cellular network fails.**

A real-time dispatch platform for emergency responders (ambulance, fire, rescue). When
infrastructure fails, field units fall back to an ad hoc mesh network (MANET) and keep relaying
incidents peer-to-peer. When connectivity returns, they sync and reconcile automatically. The mesh
layer is driven by the AURA-MANET ns-3 simulation.

> 🚧 Work in progress.

## Getting started

Requires Go 1.27+ and Docker (on macOS: `brew install go colima docker docker-compose && colima start`).

```sh
make up                 # Postgres+PostGIS and Redis via Docker Compose
make run                # backend on :8080, applies migrations on start
make test               # unit tests
make test-integration   # integration tests against a throwaway PostGIS container
```

```sh
curl -XPOST localhost:8080/incidents \
  -d '{"type":"building_collapse","lat":22.57,"lon":88.36,"people_affected":3}'
curl -XPOST localhost:8080/incidents/<id>/transition -d '{"to":"TRIAGED","priority":1}'
curl localhost:8080/incidents/<id>/events

curl -XPOST localhost:8080/responders -d '{"type":"ambulance","capabilities":["als","trauma"]}'
curl 'localhost:8080/responders?status=AVAILABLE'
```

Design decisions are recorded in [docs/adr/](docs/adr/).
