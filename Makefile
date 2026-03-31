# Make file for managing the appointment scheduler application
.PHONY: help db-up db-down schema build run stop logs clean

# Default target
help:
	@echo "Available commands:"
	@echo "  make db-up     - Start PostgreSQL container"
	@echo "  make schema    - Apply database schema"
	@echo "  make build     - Build Docker image"
	@echo "  make run       - Run the application container"
	@echo "  make stop      - Stop and remove containers"
	@echo "  make logs      - Show application logs"
	@echo "  make clean     - Full cleanup (containers + volumes)"
	@echo ""

# Start PostgreSQL
db-up:
	docker run -d \
		--name appt-scheduler-db \
		-e POSTGRES_USER=postgres \
		-e POSTGRES_PASSWORD=postgres \
		-e POSTGRES_DB=appt_scheduler \
		-p 5432:5432 \
		postgres:16
	@echo "PostgreSQL started on port 5432"

# Stop and remove PostgreSQL
db-down:
	docker stop appt-scheduler-db 2>/dev/null || true
	docker rm appt-scheduler-db 2>/dev/null || true
	@echo "PostgreSQL stopped and removed"

# Apply schema to database
schema:
	docker cp schema.sql appt-scheduler-db:/tmp/schema.sql 2>/dev/null || echo "Warning: schema.sql not found"
	@echo "Waiting for PostgreSQL to be ready..."
	docker exec appt-scheduler-db bash -c '\
		until pg_isready -h localhost -U postgres -d appt_scheduler -q; do \
			echo "Postgres is not ready yet... waiting 2s"; \
			sleep 2; \
		done; \
		echo "PostgreSQL is ready! Running schema..."; \
		psql -h localhost -U postgres -d appt_scheduler -f /tmp/schema.sql'
		
# Build Docker image
build:
	docker build -t appt-scheduler:latest .
	@echo "Docker image built: appt-scheduler:latest"

# Run the application
run:
	docker rm -f appt-scheduler-app 2>/dev/null || true
	docker run -d \
		-p 8080:8080 \
		--name appt-scheduler-app \
		-e PG_CONNECTION_STRING="postgres://postgres:postgres@host.docker.internal:5432/appt_scheduler?sslmode=disable" \
		-e ENV=local \
		appt-scheduler:latest
	@echo "App running on http://localhost:8080"

# Stop the application container
stop:
	docker stop appt-scheduler-app 2>/dev/null || true
	docker rm appt-scheduler-app 2>/dev/null || true
	@echo "App container stopped and removed"

# Show application logs
logs:
	docker logs -f appt-scheduler-app


# Full cleanup
clean:
	docker stop appt-scheduler-app appt-scheduler-db 2>/dev/null || true
	docker rm appt-scheduler-app appt-scheduler-db 2>/dev/null || true
	docker volume rm $$(docker volume ls -q | grep appt) 2>/dev/null || true
	@echo "All containers and volumes cleaned"

# Restart everything
restart: stop db-down build db-up schema run
	@echo "Full restart completed"