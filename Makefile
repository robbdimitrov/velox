.DEFAULT_GOAL := all

.PHONY: all help proto proto-check db-check k8s-check format lint test build \
	apigateway-image orderservice-image seatservice-image viewservice-image frontend-image database-image \
	deploy deploy-dry-run

KUBECTL ?= kubectl
IMAGE_REGISTRY ?= ghcr.io/example/velox
IMAGE_TAG ?= dev
PUBLIC_GATEWAY_BASE_URL ?= http://localhost:8081

all: apigateway-image orderservice-image seatservice-image viewservice-image frontend-image database-image

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
	@printf '  make *-image         Build a service image when Dockerfiles exist\n'
	@printf '  make deploy-dry-run  Server-side dry-run deploy\n'
	@printf '  make deploy          Apply Kubernetes manifests\n'

proto: proto-check

proto-check:
	@/bin/test -s pkg/pb/velox.proto
	@/usr/bin/grep -q 'package velox.v1;' pkg/pb/velox.proto

db-check:
	@/bin/test -s apps/database/migrations/001_init_logical_schemas.sql
	@/bin/test -s apps/database/seeds/999_demo_reservation_mvp.sql

k8s-check:
	@$(KUBECTL) apply --dry-run=client -f deploy/namespace.yaml
	@$(KUBECTL) apply --dry-run=client -f deploy/database.yaml
	@$(KUBECTL) apply --dry-run=client -f deploy/broker.yaml
	@$(KUBECTL) apply --dry-run=client -f deploy/cache.yaml
	@$(KUBECTL) apply --dry-run=client -f deploy/services.yaml

format:
	@gofmt -w apps/apigateway/main.go apps/apigateway/api/*.go apps/apigateway/internal/*.go apps/orderservice/internal/*.go apps/viewservice/internal/*.go
	@cargo fmt --manifest-path apps/seatservice/Cargo.toml

lint: proto-check db-check
	@sh -n scripts/deploy.sh
	@cd apps/apigateway && go test ./... >/dev/null
	@cd apps/orderservice && go test ./... >/dev/null
	@cd apps/viewservice && go test ./... >/dev/null
	@cargo test --manifest-path apps/seatservice/Cargo.toml >/dev/null
	@cd apps/frontend && npm run check >/dev/null
	@cd apps/frontend && npm run lint >/dev/null

test:
	@cd apps/apigateway && go test ./...
	@cd apps/orderservice && go test ./...
	@cd apps/viewservice && go get github.com/twmb/franz-go/pkg/kgo && go mod tidy && go test ./...
	@cargo test --manifest-path apps/seatservice/Cargo.toml

build:
	@cd apps/apigateway && go build ./...
	@cd apps/orderservice && go test ./...
	@cd apps/viewservice && go test ./...
	@cargo test --manifest-path apps/seatservice/Cargo.toml
	@cd apps/frontend && npm run build

frontend-image:
	@docker build --build-arg PUBLIC_GATEWAY_BASE_URL=$(PUBLIC_GATEWAY_BASE_URL) -t $(IMAGE_REGISTRY)-frontend:$(IMAGE_TAG) apps/frontend

database-image:
	@docker build -t $(IMAGE_REGISTRY)-database:$(IMAGE_TAG) apps/database

apigateway-image:
	@docker build -t $(IMAGE_REGISTRY)-apigateway:$(IMAGE_TAG) apps/apigateway

orderservice-image:
	@docker build -t $(IMAGE_REGISTRY)-orderservice:$(IMAGE_TAG) apps/orderservice

seatservice-image:
	@docker build -t $(IMAGE_REGISTRY)-seatservice:$(IMAGE_TAG) apps/seatservice

viewservice-image:
	@docker build -t $(IMAGE_REGISTRY)-viewservice:$(IMAGE_TAG) apps/viewservice

deploy-dry-run:
	@DRY_RUN=1 scripts/deploy.sh

deploy:
	@scripts/deploy.sh
