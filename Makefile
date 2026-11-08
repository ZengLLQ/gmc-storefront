.PHONY: dev build migrate seed gen gen-check lint test contract-test smoke smoke-admin e2e verify

DATABASE_URL ?= postgres://gmc:gmc@localhost:5432/gmc?sslmode=disable

dev:
	docker compose up -d postgres
	$(MAKE) migrate
	$(MAKE) seed
	@echo "Run 'cd api && go run ./cmd/api serve' and 'cd web && npm run dev'"

build:
	cd api && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/api ./cmd/api
	cd api && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/seed ./cmd/seed
	cd web && npm run build

migrate:
	goose -dir api/db/migrations postgres "$(DATABASE_URL)" up

seed:
	cd api && go run ./cmd/seed -csv ./db/seed/products.csv

gen:
	cd api && oapi-codegen -config oapi-codegen.yaml openapi.yaml
	cd api && sqlc generate
	cd web && npm run gen:api

gen-check: gen
	git diff --exit-code api/internal/gen web/src/lib/api/schema.d.ts

lint:
	cd web && npx @redocly/cli lint ../api/openapi.yaml
	cd api && go vet ./...
	cd web && npm run lint
	cd web && npx tsc --noEmit

test:
	cd api && go test -race -coverprofile=coverage.out ./...

contract-test:
	cd api && go test -v ./internal/platform/contract_test.go

smoke:
	bash scripts/smoke.sh

smoke-admin:
	bash scripts/smoke-admin.sh

e2e:
	cd web && npx playwright test

verify: lint gen-check test smoke smoke-admin e2e
	@echo "All verification checks passed!"
