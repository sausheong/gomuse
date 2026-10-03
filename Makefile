# Muse - build, run and test tasks. Run `make` or `make help` for a list.

BINARY  := muse
BIN_DIR := bin
CMD     := ./cmd/muse
SCORE   ?= scores/scale
ADDR    ?= 0.0.0.0:8888
DATA    ?= data
GOOS    ?= $(shell go env GOOS)
GOARCH  ?= $(shell go env GOARCH)

.DEFAULT_GOAL := help

.PHONY: help build build-convert convert build-linux run serve render test race cover vet fmt fmt-check tidy check clean

help: ## Show this help
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z_-]+:.*## / {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build bin/muse for this machine
	go build -trimpath -o $(BIN_DIR)/$(BINARY) $(CMD)

build-convert: ## Build bin/muse-convert, the sheet music converter
	go build -trimpath -o $(BIN_DIR)/muse-convert ./cmd/muse-convert

convert: build-convert ## Convert sheet music: make convert PAGES="notation/misty_*.jpeg" OUT=scores/misty.yaml
	$(BIN_DIR)/muse-convert -o $(OUT) $(PAGES)

build-linux: ## Cross-compile bin/muse-linux-amd64 for deployment
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -o $(BIN_DIR)/$(BINARY)-linux-amd64 $(CMD)

run: build ## Render one score: make run SCORE=scores/tears
	$(BIN_DIR)/$(BINARY) $(SCORE)

serve: build ## Start the web app: make serve [ADDR=:8888] [DATA=data]
	$(BIN_DIR)/$(BINARY) -s -addr $(ADDR) -data $(DATA)

render: build ## Render every sample score to scores/*.wav
	@for f in scores/*.yaml; do $(BIN_DIR)/$(BINARY) $${f%.yaml} || exit 1; done

test: ## Run the tests
	go test ./...

race: ## Run the tests with the race detector
	go test -race ./...

cover: ## Run the tests and open a coverage report
	go test -coverprofile=$(BIN_DIR)/coverage.out ./...
	go tool cover -html=$(BIN_DIR)/coverage.out

vet: ## Run go vet
	go vet ./...

fmt: ## Format the code
	gofmt -w .

fmt-check: ## Fail if any file needs formatting
	@test -z "$$(gofmt -l .)" || (echo "needs gofmt:"; gofmt -l .; exit 1)

tidy: ## Tidy go.mod and go.sum
	go mod tidy

check: fmt-check vet test ## Run everything CI should: format check, vet, tests

clean: ## Remove build output and rendered tunes
	rm -rf $(BIN_DIR)
	rm -f scores/*.wav
