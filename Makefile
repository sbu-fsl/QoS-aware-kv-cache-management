MODULE  := github.com/sbu-fsl/qos-aware-restoration
BIN_DIR := bin
QOS_BIN := $(BIN_DIR)/qos
RUN_BIN := $(BIN_DIR)/autorun

CONFIG   ?= config.yaml
DATA     ?= data
CMD_YAML ?= cmd.yaml
OUT      ?= out.txt

.PHONY: all build build-qos build-autorun run autorun clean test vet fmt help

all: build

## build: compile both binaries
build: build-qos build-autorun

build-qos:
	@mkdir -p $(BIN_DIR)
	go build -o $(QOS_BIN) ./cmd/qos

build-autorun:
	@mkdir -p $(BIN_DIR)
	go build -o $(RUN_BIN) ./cmd/autorun

## run: start the interactive QoS CLI
run: build-qos
	$(QOS_BIN) -config $(CONFIG) -data $(DATA)

## autorun: execute operations from cmd.yaml and write full report to out.txt
autorun: build-autorun
	$(RUN_BIN) -config $(CONFIG) -data $(DATA) -cmd $(CMD_YAML) -out $(OUT)

## test: run all tests
test:
	go test ./...

## vet: run go vet
vet:
	go vet ./...

## fmt: format all Go source files
fmt:
	go fmt ./...

## clean: remove compiled binaries and output file
clean:
	rm -rf $(BIN_DIR) $(OUT) $(DATA)

## help: list available targets
help:
	@grep -E '^##' Makefile | sed 's/## /  /'
