# Smart Door Security — Backend: How to Run

## Prerequisites

- [Go 1.21+](https://go.dev/dl/)
- [Docker & Docker Compose](https://docs.docker.com/get-docker/)
- [Git](https://git-scm.com/)

---

## Option 1: Run with Docker Compose (Recommended)

This starts PostgreSQL, Mosquitto (MQTT broker), and the backend all at once. Media storage is handled via Cloudinary.

```bash
docker compose up --build
```

The backend will be available at `http://localhost:8080`.

To stop all services:

```bash
docker compose down
```

---

## Option 2: Run Locally (Manual Setup)

### 1. Start Infrastructure Services

Start PostgreSQL and Mosquitto via Docker:

```bash
docker compose up postgres mosquitto
```

### 2. Configure Environment Variables

Create a `.env` file in the project root (already present if cloned):

```env
DB_URL=postgres://postgres:dipu123@localhost:5432/smartdoor
MQTT_BROKER=tcp://localhost:1883
PORT=8080
JWT_SECRET=supersecret
```

> Change `DB_URL` credentials if your PostgreSQL setup differs.

### 3. Apply Database Migrations

Connect to PostgreSQL and run the migration script:

```bash
psql -h localhost -U admin -d smartdoor -f scripts/migrate.sql
```

> Default credentials from `docker-compose.yml`: user `admin`, password `admin`, database `smartdoor`.

### 4. Install Go Dependencies

```bash
go mod download
```

### 5. Run the Server

```bash
go run ./cmd/server/main.go
```

The API will be available at `http://localhost:8080`.

---

## API Documentation (Swagger)

Once the server is running, open your browser at:

```
http://localhost:8080/swagger/index.html
```

To regenerate Swagger docs after changing annotations:

```bash
swag init -g cmd/server/main.go
```

---

## Service Ports

| Service    | Port |
| ---------- | ---- |
| Backend    | 8080 |
| PostgreSQL | 5432 |
| MQTT       | 1883 |
| Cloudinary | API  |

---

## Project Structure

```
cmd/server/         # Application entry point
internal/
  api/              # HTTP router and server setup
  config/           # Environment configuration loader
  controllers/      # Request handlers
  database/         # PostgreSQL connection
  models/           # GORM data models
  mqtt/             # MQTT client, publisher, subscriber
  repository/       # Database access layer
  services/         # Business logic
  storage/          # Cloudinary media storage
  utils/            # JWT helpers and utilities
  webrtc/           # WebRTC peer and signalling
scripts/
  migrate.sql       # Database schema
docs/               # Auto-generated Swagger docs
```

---

## Environment Variables Reference

| Variable      | Description                     | Example                                         |
| ------------- | ------------------------------- | ----------------------------------------------- |
| `DB_URL`      | PostgreSQL connection string    | `postgres://user:pass@localhost:5432/smartdoor` |
| `MQTT_BROKER` | MQTT broker address             | `tcp://localhost:1883`                          |
| `PORT`        | HTTP server port                | `8080`                                          |
| `JWT_SECRET`  | Secret key used for JWT signing | `supersecret`                                   |
