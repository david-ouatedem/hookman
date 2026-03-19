.PHONY: build dev test test-integration lint migrate migrate-down generate docker-build clean

BINARY := bin/hookman

build:
	go build -ldflags="-s -w" -o $(BINARY) ./cmd/hookman

dev:
	air

test:
	go test ./... -count=1 -short

test-integration:
	go test ./... -count=1 -tags=integration

lint:
	golangci-lint run ./...

migrate:
	migrate -path migrations -database "$(DATABASE_URL)" up

migrate-down:
	migrate -path migrations -database "$(DATABASE_URL)" down 1

generate:
	sqlc generate

docker-build:
	docker build -t hookman .

clean:
	rm -rf bin/
