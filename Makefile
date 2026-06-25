.PHONY: help proto proto-check db-check k8s-check format lint test build \
	apigateway-image orderservice-image inventoryservice-image projectionservice-image frontend-image \
	deploy deploy-dry-run

KUBECTL ?= rtk kubectl
IMAGE_REGISTRY ?= ghcr.io/example/velox
IMAGE_TAG ?= dev

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
	@rtk /bin/test -s pkg/pb/velox.proto
	@rtk /usr/bin/grep -q 'package velox.v1;' pkg/pb/velox.proto

db-check:
	@rtk /bin/test -s apps/database/migrations/001_init_logical_schemas.sql
	@rtk /bin/test -s apps/database/seeds/001_demo_reservation_mvp.sql

k8s-check:
	@$(KUBECTL) apply --dry-run=client -f deploy/k8s/namespace.yaml
	@$(KUBECTL) apply --dry-run=client -f deploy/k8s/postgres.yaml
	@$(KUBECTL) apply --dry-run=client -f deploy/k8s/redpanda.yaml
	@$(KUBECTL) apply --dry-run=client -f deploy/k8s/dragonfly.yaml
	@$(KUBECTL) apply --dry-run=client -f deploy/k8s/services.yaml

format:
	@rtk gofmt -w apps/apigateway/main.go apps/apigateway/internal/*.go apps/orderservice/internal/*.go apps/projectionservice/internal/*.go
	@rtk cargo fmt --manifest-path apps/inventoryservice/Cargo.toml

lint: proto-check db-check
	@rtk sh -n scripts/deploy.sh
	@cd apps/apigateway && rtk go test ./... >/dev/null
	@cd apps/orderservice && rtk go test ./... >/dev/null
	@cd apps/projectionservice && rtk go test ./... >/dev/null
	@rtk cargo test --manifest-path apps/inventoryservice/Cargo.toml >/dev/null

test:
	@cd apps/apigateway && rtk go test ./...
	@cd apps/orderservice && rtk go test ./...
	@cd apps/projectionservice && rtk go test ./...
	@rtk cargo test --manifest-path apps/inventoryservice/Cargo.toml

build:
	@cd apps/apigateway && rtk go build ./...
	@cd apps/orderservice && rtk go test ./...
	@cd apps/projectionservice && rtk go test ./...
	@rtk cargo test --manifest-path apps/inventoryservice/Cargo.toml

frontend-image:
	@rtk docker build -t $(IMAGE_REGISTRY)-frontend:$(IMAGE_TAG) apps/frontend

apigateway-image:
	@rtk docker build -t $(IMAGE_REGISTRY)-apigateway:$(IMAGE_TAG) apps/apigateway

orderservice-image:
	@rtk docker build -t $(IMAGE_REGISTRY)-orderservice:$(IMAGE_TAG) apps/orderservice

inventoryservice-image:
	@rtk docker build -t $(IMAGE_REGISTRY)-inventoryservice:$(IMAGE_TAG) apps/inventoryservice

projectionservice-image:
	@rtk docker build -t $(IMAGE_REGISTRY)-projectionservice:$(IMAGE_TAG) apps/projectionservice

deploy-dry-run:
	@DRY_RUN=1 scripts/deploy.sh

deploy:
	@scripts/deploy.sh
