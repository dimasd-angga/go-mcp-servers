SERVERS := filesystem postgres shell redis http homeassistant

.PHONY: build-all test-all test-shared lint clean verify smoke coverage run-% docker-build help

help:
	@echo "Targets:"
	@echo "  build-all       Build all server binaries into ./bin/"
	@echo "  test-all        Run unit tests across the workspace"
	@echo "  test-shared     Run shared/ tests only"
	@echo "  lint            Run golangci-lint"
	@echo "  smoke           Run scripts/smoke/*.sh against built binaries"
	@echo "  coverage        Run tests with coverage summary per server"
	@echo "  verify          lint + test-all + smoke (the release gate)"
	@echo "  run-<server>    go run a single server"
	@echo "  clean           Remove ./bin/"

build-all:
	@mkdir -p bin
	@for s in $(SERVERS); do \
		echo "→ build $$s"; \
		(cd servers/$$s && go build -o ../../bin/mcp-$$s .) || exit 1; \
	done

test-shared:
	@(cd shared && go test -race ./...)

test-all:
	@(cd shared && go test -race ./...) || exit 1
	@for s in $(SERVERS); do \
		echo "→ test $$s"; \
		(cd servers/$$s && go test -race ./...) || exit 1; \
	done

lint:
	@golangci-lint run ./...

smoke: build-all
	@for s in $(SERVERS); do \
		if [ -x scripts/smoke/$$s.sh ]; then \
			echo "→ smoke $$s"; \
			bash scripts/smoke/$$s.sh || exit 1; \
		fi; \
	done

coverage:
	@(cd shared && go test -coverprofile=cover.out ./... > /dev/null && \
		echo "shared: $$(go tool cover -func=cover.out | tail -1)")
	@for s in $(SERVERS); do \
		(cd servers/$$s && go test -coverprofile=cover.out ./... > /dev/null 2>&1 && \
			echo "$$s: $$(go tool cover -func=cover.out | tail -1)") || true; \
	done

verify: lint test-all smoke
	@echo "✓ verify passed"

run-%:
	@(cd servers/$* && go run .)

clean:
	@rm -rf bin/ shared/cover.out servers/*/cover.out

docker-build:
	@for s in $(SERVERS); do \
		echo "→ docker build $$s"; \
		docker build -f deploy/Dockerfile.template \
			--build-arg SERVER=$$s \
			-t go-mcp-servers/$$s:latest . || exit 1; \
	done
