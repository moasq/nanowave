.PHONY: build install clean test deps run release-snapshot skills-source-validate validate-app validate-lines e2e

BINARY_NAME=nanowave
BUILD_DIR=./bin
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-s -w -X github.com/moasq/nanowave/internal/commands.Version=$(VERSION)

build:
	go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/nanowave

install: build
	mkdir -p $(HOME)/.local/bin
	cp $(BUILD_DIR)/$(BINARY_NAME) $(HOME)/.local/bin/$(BINARY_NAME)

clean:
	rm -rf $(BUILD_DIR) dist

test:
	go test ./... -v

skills-source-validate:
	go test ./internal/orchestration -run '^TestSourceSkillsAnthropicComplianceStrict$$' -count=1 -v

deps:
	go mod tidy

run:
	go run ./cmd/nanowave $(ARGS)

release-snapshot:
	goreleaser release --snapshot --clean

validate-app:
	@if [ -z "$(PROJECT_DIR)" ] || [ -z "$(APP_NAME)" ]; then \
		echo "Usage: make validate-app PROJECT_DIR=<dir> APP_NAME=<name>"; \
		exit 1; \
	fi
	./scripts/validate-app.sh $(PROJECT_DIR) $(APP_NAME)

validate-lines:
	@if [ -z "$(PROJECT_DIR)" ]; then \
		echo "Usage: make validate-lines PROJECT_DIR=<dir>"; \
		exit 1; \
	fi
	./scripts/validate-line-limits.sh $(PROJECT_DIR)

e2e:
	./claude-docker/test-e2e.sh
