MODULE  := github.com/sbu-fsl/qos-aware-restoration
BIN_DIR := bin
APP_BIN := $(BIN_DIR)/cli

CONFIG   ?= config.yaml
DATA     ?= data
CMD_YAML ?= cmd.yaml
OUT      ?= out.txt

.PHONY: all build build-go run autorun clean test vet fmt help

all: build

## build: compile both binaries
build: build-go

build-go:
	@mkdir -p $(BIN_DIR)
	go build -o $(APP_BIN) .

## run: start the interactive QoS CLI
run: build-go
	$(APP_BIN) qos --config $(CONFIG) --data $(DATA)

## autorun: execute operations from cmd.yaml and write full report to out.txt
autorun: build-go
	$(APP_BIN) autorun --config $(CONFIG) --data $(DATA) --cmd $(CMD_YAML) --out $(OUT)

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
