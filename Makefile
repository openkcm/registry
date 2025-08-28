.PHONY: default test

default: test

test: install-gotestsum
	rm -f cover.* junit.xml
	env TEST_ENV=make gotestsum --format testname --junitfile junit.xml -- -coverprofile cover.out ./internal/...

test-cover: test
	go tool cover -html=cover.out

# installs gotestsum test helper
install-gotestsum:
	(cd /tmp && go install gotest.tools/gotestsum@latest)

# compiles service test proto file into corresponding Go source files
compile-servicetest-pb:
	protoc --go_out=./internal/interceptor/servicetest --go_opt=module=github.com/openkcm/registry/internal/interceptor/servicetest \
		--go-grpc_out=./internal/interceptor/servicetest --go-grpc_opt=module=github.com/openkcm/registry/internal/interceptor/servicetest internal/interceptor/servicetest/servicetest.proto

# Builds the registry binary for Linux AMD64 architecture. Needed for Docker image creation.
go-build-for-docker:
	GOOS=linux GOARCH=amd64 go build -trimpath ./cmd/registry

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

# Prerequisite: PostgreSQL needs to be running
int-test-up-and-run:
	$(MAKE) go-build-and-run
	-$(MAKE) int-test-run
	$(MAKE) go-stop-and-remove

# Prerequisite: PostgreSQL and Registry Service need to be running
int-test-run:
	go test -v -count=1 -parallel=5 -race -shuffle=on ./integration/... -tags=integration

# Prerequisite: PostgreSQL needs to be running
int-test-up-and-run-cover:
	mkdir -p cover
	$(MAKE) go-build-and-run cover_flag=-cover cover_dir_env=GOCOVERDIR=cover
	-$(MAKE) int-test-run
	$(MAKE) go-stop-and-remove
	go tool covdata textfmt -i=./cover -o cover.out
	rm -r ./cover
	go tool cover -html=cover.out


# This target is used to run all tests and create a merged coverage report from unit and integration tests
# Prerequisite: PostgreSQL needs to be running
all-tests-run-cover:
	mkdir -p cover/integration
	mkdir -p cover/unit
	$(MAKE) go-build-and-run cover_flag=-cover cover_dir_env=GOCOVERDIR=cover/integration
	-$(MAKE) int-test-run
	$(MAKE) go-stop-and-remove
	echo "Running unit tests"
	go test -cover ./internal/... -args -test.gocoverdir="${PWD}/cover/unit"
	echo "Creating coverage report"
	go tool covdata textfmt -i=./cover/unit,./cover/integration -o cover.out
	go tool cover --html=cover.out -o cover.html
	rm -r ./cover

go-build-and-run:
	go build $(cover_flag) -o registry ./cmd/registry
	$(cover_dir_env) ./registry 1>/dev/null 2>/dev/null & echo $$! > pid.txt

go-stop-and-remove:
	kill -2 `cat pid.txt` && rm pid.txt
	rm registry

clean-docker-compose:
	docker compose down -v && docker compose rm -f -v

# Runs unit and integration tests with coverage, starts dependency containers, and generates coverage report
integration-test: docker-compose-dependencies-up all-tests-run-cover

generate-certs:
	(cd local/rabbitmq && chmod +x generate-certs.sh && ./generate-certs.sh)

.PHONY: lint
lint:
	golangci-lint run -v --fix ./...

# Helm chart tests using existing K8s cluster
helm-test: install-gotestsum
	@echo "Running Helm chart tests on existing Kubernetes cluster..."
	env TEST_ENV=make gotestsum --format testname -- -tags=helmtest -timeout=20m ./helmtest/...

# Install dependencies with Helm
helm-install-dependencies: generate-certs
	./helm_dependencies.sh install

# Uninstall dependencies with Helm
helm-uninstall-dependencies:
	./helm_dependencies.sh uninstall

helm-install-registry: docker-build
	./helm_registry.sh install

# Uninstalls the registry-service from the local Kubernetes cluster using Helm.
helm-uninstall-registry:
	./helm_registry.sh uninstall