SHELL := C:/Program Files/Git/bin/sh.exe
.SHELLFLAGS := -eu -o pipefail -c

ROOT := $(shell pwd)
API_DIR := apps/api
WEB_DIR := apps/web
FIRMWARE_DIR := firmware/esp32-display

GO ?= $(shell if [ -x "$$(command -v go 2>/dev/null)" ]; then command -v go; elif [ -x "/c/Program Files/Go/bin/go.exe" ]; then echo "/c/Program Files/Go/bin/go.exe"; else echo go; fi)
PYTHON ?= $(shell if [ -x "$$(command -v python 2>/dev/null)" ]; then command -v python; elif [ -x "$$(command -v python3 2>/dev/null)" ]; then command -v python3; elif [ -x "/c/Python314/python.exe" ]; then echo "/c/Python314/python.exe"; else echo python; fi)
PIO_CANDIDATE := $(shell ls -d "$$HOME"/AppData/Roaming/Python/Python3*/Scripts/pio.exe 2>/dev/null | sort -V | tail -1)
ifeq ($(strip $(PIO_CANDIDATE)),)
  PIO ?= $(PYTHON) -m platformio
else
  PIO ?= $(PIO_CANDIDATE)
endif

API_ADDR ?= 127.0.0.1:8080
UPLOAD_PORT ?= COM3

.DEFAULT_GOAL := help
.PHONY: help install install-api install-web install-firmware dev dev-api dev-web build build-api build-web build-firmware test test-api test-web lint lint-api lint-web typecheck clean clean-api clean-web clean-firmware format format-api format-web fmt-firmware run-api run-web flash monitor firmware-serial tune-gc clean-gc db-reset

help: ## Show this help
	@echo "prodi"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

install: install-api install-web install-firmware ## Install all dependencies

install-api: ## Download Go module dependencies
	cd $(API_DIR) && "$(GO)" mod download

install-web: ## Install npm dependencies
	npm install

install-firmware: ## Install PlatformIO Core
	"$(PYTHON)" -m pip install --user platformio
	"$(PYTHON)" -m platformio --version

dev: ## Run API and web dev servers together
	@echo "API  -> http://$(API_ADDR)"
	@echo "Web  -> http://localhost:5173"
	@trap 'kill 0' EXIT INT TERM; \
	(cd $(API_DIR) && "$(GO)" run ./cmd/prodi-api) & \
	(cd $(WEB_DIR) && npm run dev) & \
	wait

dev-api: ## Run the API only
	cd $(API_DIR) && "$(GO)" run ./cmd/prodi-api

dev-web: ## Run the web app only
	cd $(WEB_DIR) && npm run dev

build: build-api build-web build-firmware ## Build every target

build-api: ## Build the API binary
	cd $(API_DIR) && "$(GO)" build -o bin/prodi-api ./cmd/prodi-api

build-web: ## Build the web app
	cd $(WEB_DIR) && npm run build

build-firmware: ## Build the ESP32 firmware
	cd $(FIRMWARE_DIR) && "$(PIO)" run

test: test-api test-web ## Run all tests

test-api: ## Run Go tests
	cd $(API_DIR) && "$(GO)" test ./...

test-web: ## Typecheck the web app
	cd $(WEB_DIR) && npm run typecheck

lint: lint-api lint-web ## Lint every target

lint-api: ## Vet the Go code
	cd $(API_DIR) && "$(GO)" vet ./...

lint-web: ## Lint the web app
	cd $(WEB_DIR) && npm run lint

typecheck: ## Typecheck the web app
	cd $(WEB_DIR) && npm run typecheck

format: format-api format-web ## Format source code

format-api: ## Format the Go code
	cd $(API_DIR) && "$(GO)" fmt ./...

format-web: ## Format the web app
	cd $(WEB_DIR) && npm run format

fmt-firmware: ## Format the firmware sources
	cd $(FIRMWARE_DIR) && "$(PYTHON)" -m platformio run -t clean

run-api: build-api ## Build and run the API
	cd $(API_DIR) && ./bin/prodi-api

run-web: ## Preview the production web build
	cd $(WEB_DIR) && npm run preview

flash: ## Upload the firmware to the ESP32
	cd $(FIRMWARE_DIR) && "$(PIO)" run -t upload --upload-port $(UPLOAD_PORT)

monitor: ## Open the ESP32 serial monitor
	cd $(FIRMWARE_DIR) && "$(PIO)" device monitor --port $(UPLOAD_PORT)

firmware-serial: ## List serial devices and exit
	cd $(FIRMWARE_DIR) && "$(PYTHON)" -m platformio device list

tune-gc: ## Clean unneeded firmware files
	cd $(FIRMWARE_DIR) && "$(PYTHON)" -m platformio run -t clean

clean-gc: ## Deep clean firmware build
	cd $(FIRMWARE_DIR) && "$(PYTHON)" -m platformio run -t fullclean

db-reset: ## Delete the local SQLite database
	rm -f $(API_DIR)/data/prodi.db $(API_DIR)/data/prodi.db-shm $(API_DIR)/data/prodi.db-wal
	@echo "database removed"

clean: clean-api clean-web clean-firmware ## Remove build artifacts

clean-api:
	rm -rf $(API_DIR)/bin

clean-web:
	rm -rf $(WEB_DIR)/dist $(WEB_DIR)/node_modules/.vite

clean-firmware:
	rm -rf $(FIRMWARE_DIR)/.pio
