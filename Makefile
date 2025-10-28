ifneq (,$(wildcard .env))
include .env
export DB_DSN
endif

.PHONY: run migrate-up migrate-down lint

run:
	go run ./cmd/bot

migrate-up:
	goose -dir ./migrations postgres "$(DB_DSN)" up

migrate-down:
	goose -dir ./migrations postgres "$(DB_DSN)" down

lint:
	golangci-lint run
