SERVER_URL ?= http://localhost:9000
INNGEST_ENDPOINT = $(SERVER_URL)/api/inngest

build:
	@echo "Building..."
	@go build -o main cmd/api/main.go

run-backend:
	@go run cmd/api/main.go

docker-up:
	@if docker compose up --build 2>/dev/null; then \
		: ; \
	else \
		echo "Falling back to Docker Compose V1"; \
		docker-compose up --build; \
	fi

docker-down:
	@if docker compose down 2>/dev/null; then \
		: ; \
	else \
		echo "Falling back to Docker Compose V1"; \
		docker-compose down; \
	fi

clean:
	@echo "Cleaning..."
	@rm -f main

watch:
	@if command -v air > /dev/null; then \
            air; \
            echo "Watching...";\
        else \
            read -p "Go's 'air' is not installed on your machine. Do you want to install it? [Y/n] " choice; \
            if [ "$$choice" != "n" ] && [ "$$choice" != "N" ]; then \
                go install github.com/air-verse/air@latest; \
                air; \
                echo "Watching...";\
            else \
                echo "You chose not to install air. Exiting..."; \
                exit 1; \
            fi; \
        fi

migrate-up:
	@echo "Loading environment variables and running migrations..."
	@export $$(grep -v '^#' .env | xargs) && goose -dir pkg/database/migrations postgres "$$DB_URL" up

migrate-down:
	@echo "Loading environment variables and rolling back migrations..."
	@export $$(grep -v '^#' .env | xargs) && goose -dir pkg/database/migrations postgres "$$DB_URL" down

migrate-status:
	@echo "Loading environment variables and checking migration status..."
	@export $$(grep -v '^#' .env | xargs) && goose -dir pkg/database/migrations postgres "$$DB_URL" status

run-frontend:
	@echo "Starting frontend..."
	@cd frontend && bun run dev

run-inngest:
	@echo "Starting inngest..."
	npx inngest-cli@latest dev -u $(INNGEST_ENDPOINT)

run-auth:
	@echo "Starting auth service..."
	@cd packages/auth && bun run dev

make run-studio:
	@echo "Starting studio..."
	@cd packages/auth && bun run db:studio

.PHONY: build run-backend clean watch docker-run docker-down migrate-up migrate-down migrate-status run-frontend run-inngest run-auth run-studio
