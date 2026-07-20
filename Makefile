.PHONY: default test lint clean helm-test helm-install-dependencies helm-uninstall-dependencies helm-install-registry helm-uninstall-registry

default: test

# Runs unit + integration tests end-to-end:
#   1. brings up dependencies (postgres, rabbitmq, otel-collector)
#   2. builds the registry binary with coverage instrumentation and starts it
#   3. runs unit tests (./internal/...) with coverage into cover/unit
#   4. runs integration tests (./integration/... -tags=integration) against the running binary
#   5. stops the registry, merges unit + integration coverage into cover.out + cover.html
# Everything is torn down (registry process, docker compose stack, temp files) via a trap
# regardless of whether the recipe succeeded, failed, or was interrupted.
test: install-gotestsum generate-certs
	@rm -rf cover cover.out cover.html junit-unit.xml junit-integration.xml registry pid.txt
	@mkdir -p cover/unit cover/integration
	@set -u; \
	cleanup() { \
	  status=$$?; \
	  trap - EXIT INT TERM; \
	  echo "==> Tearing down"; \
	  if [ -f pid.txt ]; then \
	    kill -2 "$$(cat pid.txt)" 2>/dev/null || true; \
	    wait "$$(cat pid.txt)" 2>/dev/null || true; \
	    rm -f pid.txt; \
	  fi; \
	  rm -f registry; \
	  rm -rf cover; \
	  docker compose down -v --remove-orphans >/dev/null 2>&1 || true; \
	  exit $$status; \
	}; \
	trap cleanup EXIT INT TERM; \
	set -e; \
	echo "==> Bringing up dependencies"; \
	docker compose up postgres rabbitmq otel-collector -d --wait; \
	echo "==> Building registry binary (with coverage)"; \
	go build -cover -o registry ./cmd/registry; \
	echo "==> Starting registry"; \
	GOCOVERDIR=$(CURDIR)/cover/integration ./registry >/dev/null 2>&1 & echo $$! > pid.txt; \
	ready=0; \
	for i in $$(seq 1 45); do \
	  if go tool github.com/grpc-ecosystem/grpc-health-probe -addr=localhost:9092 >/dev/null 2>&1; then \
	    echo "Registry service is ready"; ready=1; break; \
	  fi; \
	  echo "Waiting for registry... ($$i/45)"; \
	  sleep 1; \
	done; \
	if [ "$$ready" -ne 1 ]; then echo "Registry service failed to start within 45 seconds"; exit 1; fi; \
	echo "==> Running unit tests"; \
	env TEST_ENV=make gotestsum --junitfile=junit-unit.xml --format=testname -- \
	    -cover ./internal/... -args -test.gocoverdir="$(CURDIR)/cover/unit"; \
	echo "==> Running integration tests"; \
	gotestsum --junitfile=junit-integration.xml --format=testname -- \
	    -v -count=1 -parallel=5 -race -shuffle=on -tags=integration ./integration/...; \
	echo "==> Stopping registry (to flush coverage)"; \
	kill -2 "$$(cat pid.txt)"; \
	wait "$$(cat pid.txt)" 2>/dev/null || true; \
	rm -f pid.txt; \
	echo "==> Merging coverage"; \
	go tool covdata textfmt -i=./cover/unit,./cover/integration -o cover.out; \
	go tool cover -html=cover.out -o cover.html

# installs gotestsum test helper
install-gotestsum:
	(cd /tmp && go install gotest.tools/gotestsum@latest)

# compiles service test proto file into corresponding Go source files
compile-servicetest-pb:
	protoc --go_out=./internal/interceptor/servicetest --go_opt=module=github.com/openkcm/registry/internal/interceptor/servicetest \
		--go-grpc_out=./internal/interceptor/servicetest --go-grpc_opt=module=github.com/openkcm/registry/internal/interceptor/servicetest internal/interceptor/servicetest/servicetest.proto

# Builds the registry binary for Linux AMD64 architecture. Needed for Docker image creation.
go-build-for-docker:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o registry ./cmd/registry

docker-build: go-build-for-docker
	docker build --no-cache -f Dockerfile.dev -t registry:dev .

docker-compose-dependencies-up: generate-certs
	docker compose up postgres rabbitmq otel-collector -d --wait

docker-compose-registry-up: docker-build
	docker compose up registry -d

docker-compose-up: docker-build generate-certs
	docker compose up -d

docker-log:
	docker compose logs --tail 10 -f

# Prerequisite: see docker-compose-up
docker-compose-up-and-log: docker-compose-up
	$(MAKE) docker-log

docker-compose-dependencies-up-and-log: docker-compose-dependencies-up
	$(MAKE) docker-log

generate-certs:
	(cd local/rabbitmq && chmod +x generate-certs.sh && ./generate-certs.sh)

lint:
	golangci-lint run -v --fix ./...

# Helm chart tests using existing K8s cluster
helm-test: install-gotestsum
	@echo "Running Helm chart tests on existing Kubernetes cluster..."
	env TEST_ENV=make gotestsum --format testname -- -tags=helmtest -timeout=20m ./helmtest/...

# Install dependencies with Helm
helm-install-dependencies:
	./helm_dependencies.sh install

# Uninstall dependencies with Helm
helm-uninstall-dependencies:
	./helm_dependencies.sh uninstall

helm-install-registry:
	./helm_registry.sh install

# Uninstalls the registry-service from the local Kubernetes cluster using Helm.
helm-uninstall-registry:
	./helm_registry.sh uninstall

clean:
	rm -f junit-unit.xml junit-integration.xml
	rm -f cover.out cover.html
	rm -rf ./cover/
	rm -f registry pid.txt
