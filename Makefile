.PHONY: dev test build

dev:
	docker compose up -d control-db
	@trap 'kill 0' EXIT INT TERM; \
	go run ./control-panel-service/cmd/api & \
	go run ./node-agent-service/cmd/node & \
	(cd control-web && npm run dev) & \
	wait

test:
	docker compose up -d control-db
	go vet ./control-panel-service/... ./node-agent-service/...
	DATABASE_URL_TEST=postgres://mbr:mbr@localhost:5432/mbr?sslmode=disable go test ./control-panel-service/... ./node-agent-service/...
	cd control-web && npm run check

build:
	go build -o bin/api ./control-panel-service/cmd/api
	go build -o bin/node ./node-agent-service/cmd/node
	cd control-web && npm run build
