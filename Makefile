.PHONY: dev test build

dev:
	docker compose up -d control-db
	@trap 'kill 0' EXIT INT TERM; \
	go run ./cmd/api & \
	go run ./cmd/node & \
	(cd web && npm run dev) & \
	wait

test:
	go vet ./...
	go test ./...
	cd web && npm run check

build:
	go build -o bin/api ./cmd/api
	go build -o bin/node ./cmd/node
	cd web && npm run build
