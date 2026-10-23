SHELL := /bin/bash
ROOT := $(CURDIR)
export PATH := $(ROOT)/.runtime/go/bin:$(ROOT)/.runtime/pg/usr/bin:$(ROOT)/.runtime/redis-7.4.2/src:$(PATH)
export GOPATH := $(ROOT)/.runtime/gopath
export GOCACHE := $(ROOT)/.runtime/gocache
SCENARIO ?= race
REQUESTS ?= 100
CONCURRENCY ?= 100
.PHONY: setup services dev demo build bench test stop
setup:
	bash scripts/setup.sh
services:
	bash scripts/services.sh
build:
	mkdir -p bin
	go build -o bin/api ./cmd/api
	go build -o bin/worker ./cmd/worker
	go build -o bin/load ./cmd/load
	cd web && npm run build
dev:
	bash scripts/dev.sh
demo:
	DEMO_MODE=1 bash scripts/dev.sh
bench:
	mkdir -p results
	set -o pipefail; bin/load -scenario $(SCENARIO) -requests $(REQUESTS) -concurrency $(CONCURRENCY) | tee results/latest.json
test: services
	bash scripts/test.sh
stop:
	bash scripts/services.sh stop
