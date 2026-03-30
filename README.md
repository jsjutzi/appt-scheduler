# Appointment Scheduler API

A RESTful Go API for managing trainer/coach appointments with strict business rules (Pacific Time, 30-minute slots, no overlaps, weekdays only, no US holidays).

## Features

- View available 30-minute appointment slots
- Book new appointments
- View scheduled appointments per trainer
- Update/Reschedule appointments
- Cancel appointments
- Full Swagger UI for interactive testing
- Docker support

## Tech Stack

- Go 1.26
- PostgreSQL 16
- Chi Router
- pgx
- Swagger (Swaggo)
- Docker

---

## Quick Start (From Scratch)

### 1. Prerequisites

- Docker
- Git
- Make (recommended)
- Swag (for generating swagger docs locally without Docker)

### 2. Clone the Repository

```bash
git clone https://github.com/jsjutzi/appt-scheduler
cd appt-scheduler
```

### 4. Run Service

-- Start PostgreSQL + Build + Run the API (recommended)

```bash
make restart
```

-- Or run step by step:

```bash
make db-up      # Start PostgreSQL
make schema     # Create database tables
make build      # Build Docker image
make run        # Start the API
```

### 5. Consuming the Service

-- You can access swagger api docs locally once the application is running:
`http://localhost:8080/swagger/index.html#/`