.PHONY: default test

# Extract Go version from go.mod
GO_VERSION := $(shell grep '^go ' go.mod | awk '{print $$2}')

# get git values
squash: HEAD := $(shell git rev-parse HEAD)
squash: CURRENT_BRANCH := $(shell git branch --show-current)
squash: MERGE_BASE := $(shell git merge-base origin/main $(CURRENT_BRANCH))

default: test

test: install-gotestsum
	rm -f cover.* junit.xml
	env TEST_ENV=make gotestsum --format testname --junitfile junit.xml -- -coverprofile cover.out ./internal/...

test-cover: test
	go tool cover -html=cover.out

# installs gotestsum test helper
install-gotestsum:
	(cd /tmp && go install gotest.tools/gotestsum@latest)

# squash will take all commits on the current branch (commits done after branched away from main) and squash them into a single commit on top of main HEAD.
# The commit message of this single commit is compiled from all previous commits. Please modify as needed.
# After all: force push to origin.
squash:
	@git diff --quiet || (echo "you have untracked changes, stopping" && exit 1)
	@echo "*********** if anything goes wrong, you can simply reset all the changes made executing: git reset --hard $(HEAD)"
	git log --pretty=format:"%+* %B" --reverse $(MERGE_BASE).. > git_log.tmp
	git branch safe/$(CURRENT_BRANCH)
	git reset $(MERGE_BASE)
	git stash
	git reset --hard origin/main
	git stash apply
	git add .
	git commit -F git_log.tmp
	@rm -f git_log.tmp

# compiles service test proto file into corresponding Go source files
compile-servicetest-pb:
	protoc --go_out=./internal/interceptor/servicetest --go_opt=module=github.com/openkcm/registry/internal/interceptor/servicetest \
		--go-grpc_out=./internal/interceptor/servicetest --go-grpc_opt=module=github.com/openkcm/registry/internal/interceptor/servicetest internal/interceptor/servicetest/servicetest.proto

# env variables for local deployment
db_user = postgres
db_pass = secret
db_name = registry
db_config = DB_USER=$(db_user) DB_PASS=$(db_pass) DB_NAME=$(db_name)

OTEL_HOST=otel-collector:4317

# Builds the registry binary for Linux AMD64 architecture. Needed for Docker image creation.
go-build-for-docker:
	GOOS=linux GOARCH=amd64 go build -trimpath ./cmd/registry-service

docker-build: go-build-for-docker
	docker build --no-cache -f Dockerfile -t registry-service:dev .

docker-compose-dependencies-up: generate-certs
	$(db_config) docker compose up postgres rabbitmq -d --wait

docker-compose-registry-service-up: docker-build
	docker compose up registry-service -d

docker-compose-up: docker-build generate-certs
	$(db_config) docker compose up -d

docker-log:
	$(db_config) docker compose logs --tail 10 -f

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
	rm -r ./cover

go-build-and-run:
	go build $(cover_flag) -o registry-service ./cmd/registry-service
	$(cover_dir_env) OTEL_HOST=localhost:4317 ./registry-service 1>/dev/null 2>/dev/null & echo $$! > pid.txt

go-stop-and-remove:
	kill -2 `cat pid.txt` && rm pid.txt
	rm registry-service

clean-docker-compose:
	$(db_config) docker compose down -v && $(db_config) docker compose rm -f -v

# Required by piper pipeline in order to customize coverage report creation
sonartest: docker-compose-dependencies-up all-tests-run-cover

generate-certs:
	(cd local/rabbitmq && chmod +x generate-certs.sh && ./generate-certs.sh)


.PHONY: lint
lint:
	golangci-lint run -v --fix ./...