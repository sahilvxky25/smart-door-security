SHELL := cmd.exe
.SHELLFLAGS := /C
.DEFAULT_GOAL := help

.PHONY: help
help:
	@echo "Usage: make <target>"
	@echo ""
	@echo "Backend (Go) targets:"
	@echo "  backend-deps          Download Go deps"
	@echo "  backend-test          Run backend tests"
	@echo "  backend-fmt           Format backend Go code"
	@echo "  backend-vet           Vet backend Go code"
	@echo "  backend-build         Build backend binary into backend/bin/"
	@echo "  backend-run           Run backend with air (hot reload)"
	@echo "  backend-swagger       Regenerate Swagger docs (swag)"
	@echo "  backend-docker-up     Run backend + deps via docker compose"
	@echo "  backend-docker-down   Stop docker compose stack"
	@echo ""
	@echo "App (Flutter) targets:"
	@echo "  app-deps              flutter pub get"
	@echo "  app-test              flutter test"
	@echo "  app-format            dart format ."
	@echo "  app-analyze           flutter analyze"
	@echo "  app-run               flutter run"
	@echo "  app-build-apk         flutter build apk --release"
	@echo ""
	@echo "ESP32 (PlatformIO) targets:"
	@echo "  esp32-build           pio run"
	@echo "  esp32-upload          pio run -t upload"
	@echo "  esp32-monitor         pio device monitor"
	@echo ""
	@echo "Face service (Python) targets:"
	@echo "  face-install          pip install -r requirements.txt"
	@echo "  face-run              Run face_service.py"

BACKEND_DIR := backend
APP_DIR := application
ESP32_DIR := esp32
FACE_DIR := backend/face_service

DOCKER_COMPOSE ?= docker compose
PYTHON ?= python

.PHONY: backend-deps
backend-deps:
	cd $(BACKEND_DIR) && go mod download

.PHONY: backend-test
backend-test:
	cd $(BACKEND_DIR) && go test ./...

.PHONY: backend-fmt
backend-fmt:
	cd $(BACKEND_DIR) && go fmt ./...

.PHONY: backend-vet
backend-vet:
	cd $(BACKEND_DIR) && go vet ./...

.PHONY: backend-build
backend-build:
	cd $(BACKEND_DIR) && go build -o bin/ ./cmd/server

.PHONY: backend-run
backend-run:
	cd $(BACKEND_DIR) && air

.PHONY: backend-swagger
backend-swagger:
	cd $(BACKEND_DIR) && swag init -g cmd/server/main.go

.PHONY: backend-docker-up
backend-docker-up:
	cd $(BACKEND_DIR) && $(DOCKER_COMPOSE) up mosquitto

.PHONY: backend-docker-down
backend-docker-down:
	cd $(BACKEND_DIR) && $(DOCKER_COMPOSE) down

.PHONY: app-deps
app-deps:
	cd $(APP_DIR) && flutter pub get

.PHONY: app-test
app-test:
	cd $(APP_DIR) && flutter test

.PHONY: app-format
app-format:
	cd $(APP_DIR) && dart format .

.PHONY: app-analyze
app-analyze:
	cd $(APP_DIR) && flutter analyze

.PHONY: app-run
app-run:
	cd $(APP_DIR) && flutter run

.PHONY: app-build-apk
app-build-apk:
	cd $(APP_DIR) && flutter build apk --release

.PHONY: esp32-build
esp32-build:
	cd $(ESP32_DIR) && pio run

.PHONY: esp32-upload
esp32-upload:
	cd $(ESP32_DIR) && pio run -t upload

.PHONY: esp32-monitor
esp32-monitor:
	cd $(ESP32_DIR) && pio device monitor

.PHONY: face-install
face-install:
	cd $(FACE_DIR) && $(PYTHON) -m pip install -r requirements.txt

.PHONY: face-run
face-run:
	cd $(BACKEND_DIR) && call .venv\Scripts\activate && python face_service\face_service.py