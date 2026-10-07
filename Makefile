COMPOSE := docker compose -f deploy/docker-compose.yml

# Colima doesn't use /var/run/docker.sock on the host; point testcontainers at it.
COLIMA_SOCK := $(HOME)/.colima/default/docker.sock
ifneq ($(wildcard $(COLIMA_SOCK)),)
export DOCKER_HOST ?= unix://$(COLIMA_SOCK)
export TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE ?= /var/run/docker.sock
endif

.PHONY: up down reset run build test test-integration lint

up:                ## start Postgres + Redis
	$(COMPOSE) up -d --wait

down:              ## stop infrastructure (keeps data)
	$(COMPOSE) down

reset:             ## stop infrastructure and delete data
	$(COMPOSE) down -v

run:               ## run the backend (applies migrations on start)
	go run ./cmd/beacon

build:
	go build -o bin/beacon ./cmd/beacon

test:              ## unit tests (no Docker needed)
	go test -race ./...

test-integration:  ## integration tests against a throwaway PostGIS container
	go test -race -tags integration -count=1 ./...

lint:
	go vet ./...
	test -z "$$(gofmt -l .)" || (gofmt -l . && exit 1)
