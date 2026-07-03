.DEFAULT_GOAL := all

KUBECTL ?= kubectl
IMAGE_PREFIX ?= localhost:5000/velox
GIT_SHA ?= $(shell git rev-parse --short HEAD)
PUBLIC_GATEWAY_BASE_URL ?= http://localhost:8081

.PHONY: all
all: apigateway orderservice seatservice viewservice frontend database

.PHONY: help
help:
	@printf 'Velox support targets:\n'
	@printf '  make proto           Validate transport contracts\n'
	@printf '  make format          Run available formatters\n'
	@printf '  make lint            Run lightweight static checks\n'
	@printf '  make test            Run service tests\n'
	@printf '  make build           Run available local builds\n'
	@printf '  make proto-check     Validate protobuf text shape\n'
	@printf '  make db-check        Validate SQL files are present\n'
	@printf '  make k8s-check       Client-side validate Kubernetes YAML\n'
	@printf '  make <service>       Build and push a service image\n'
	@printf '  make deploy-dry-run  Server-side dry-run deploy\n'
	@printf '  make deploy          Apply Kubernetes manifests\n'

.PHONY: proto
proto: proto-check

.PHONY: proto-check
proto-check:
	@/bin/test -s pkg/pb/velox.proto
	@/usr/bin/grep -q 'package velox.v1;' pkg/pb/velox.proto

.PHONY: db-check
db-check:
	@/bin/test -s apps/database/migrations/001_init_logical_schemas.sql
	@/bin/test -s apps/database/seeds/999_demo_reservation_mvp.sql

.PHONY: k8s-check
k8s-check:
	@$(KUBECTL) apply --dry-run=client -f deploy/namespace.yaml
	@$(KUBECTL) apply --dry-run=client -f deploy/database.yaml
	@$(KUBECTL) apply --dry-run=client -f deploy/broker.yaml
	@$(KUBECTL) apply --dry-run=client -f deploy/cache.yaml
	@$(KUBECTL) apply --dry-run=client -f deploy/services.yaml

.PHONY: format
format:
	@gofmt -w apps/apigateway/main.go apps/apigateway/api/*.go apps/apigateway/internal/*.go apps/orderservice/internal/*.go apps/viewservice/internal/*.go
	@cargo fmt --manifest-path apps/seatservice/Cargo.toml

.PHONY: lint
lint: proto-check db-check
	@sh -n scripts/deploy.sh
	@cd apps/apigateway && go test ./... >/dev/null
	@cd apps/orderservice && go test ./... >/dev/null
	@cd apps/viewservice && go test ./... >/dev/null
	@cargo test --manifest-path apps/seatservice/Cargo.toml >/dev/null
	@cd apps/frontend && npm run check >/dev/null
	@cd apps/frontend && npm run lint >/dev/null

.PHONY: test
test:
	@echo "Testing apigateway..."
	@cd apps/apigateway && go test ./...
	@echo "Testing orderservice..."
	@cd apps/orderservice && go test ./...
	@echo "Testing viewservice..."
	@cd apps/viewservice && go test ./...
	@echo "Testing seatservice..."
	@cargo test --manifest-path apps/seatservice/Cargo.toml
	@echo "Testing frontend..."
	@cd apps/frontend && npm run test

.PHONY: build
build:
	@cd apps/apigateway && go build ./...
	@cd apps/orderservice && go build ./...
	@cd apps/viewservice && go build ./...
	@cargo build --manifest-path apps/seatservice/Cargo.toml
	@cd apps/frontend && npm run build

.PHONY: frontend
frontend:
	docker build --build-arg PUBLIC_GATEWAY_BASE_URL=$(PUBLIC_GATEWAY_BASE_URL) -t $(IMAGE_PREFIX)-frontend:$(GIT_SHA) apps/frontend
	docker push $(IMAGE_PREFIX)-frontend:$(GIT_SHA)

.PHONY: database
database:
	docker build -t $(IMAGE_PREFIX)-database:$(GIT_SHA) apps/database
	docker push $(IMAGE_PREFIX)-database:$(GIT_SHA)

.PHONY: apigateway
apigateway:
	docker build -t $(IMAGE_PREFIX)-apigateway:$(GIT_SHA) apps/apigateway
	docker push $(IMAGE_PREFIX)-apigateway:$(GIT_SHA)

.PHONY: orderservice
orderservice:
	docker build -t $(IMAGE_PREFIX)-orderservice:$(GIT_SHA) apps/orderservice
	docker push $(IMAGE_PREFIX)-orderservice:$(GIT_SHA)

.PHONY: seatservice
seatservice:
	docker build -t $(IMAGE_PREFIX)-seatservice:$(GIT_SHA) apps/seatservice
	docker push $(IMAGE_PREFIX)-seatservice:$(GIT_SHA)

.PHONY: viewservice
viewservice:
	docker build -t $(IMAGE_PREFIX)-viewservice:$(GIT_SHA) apps/viewservice
	docker push $(IMAGE_PREFIX)-viewservice:$(GIT_SHA)

.PHONY: deploy-dry-run
deploy-dry-run:
	@DRY_RUN=1 scripts/deploy.sh

.PHONY: deploy
deploy:
	@scripts/deploy.sh
