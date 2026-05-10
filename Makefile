SHELL := /bin/bash

.PHONY: dev-up dev-restart dev-down

DEV_NAME ?= cli-relay-dev
DEV_CMD ?= go run ./cmd/server
DEV_CONFIG ?= config.example.yaml
DEV_PID_FILE ?= .tmp/$(DEV_NAME).pid
DEV_LOG_FILE ?= .tmp/$(DEV_NAME).log

dev-up:
	@mkdir -p .tmp
	@if [ -f "$(DEV_PID_FILE)" ] && kill -0 "$$(cat $(DEV_PID_FILE))" >/dev/null 2>&1; then \
		echo "dev server already running (pid=$$(cat $(DEV_PID_FILE)))"; \
		exit 1; \
	fi
	@echo "starting CliRelay in local dev mode..."
	@nohup $(DEV_CMD) -config $(DEV_CONFIG) >> "$(DEV_LOG_FILE)" 2>&1 & \
		echo "$$!" > "$(DEV_PID_FILE)"
	@sleep 1
	@echo "started pid=$$(cat $(DEV_PID_FILE))"
	@echo "log: $(DEV_LOG_FILE)"

dev-restart: dev-down dev-up

dev-down:
	@if [ -f "$(DEV_PID_FILE)" ]; then \
		pid="$$(cat $(DEV_PID_FILE))"; \
		if kill -0 "$$pid" >/dev/null 2>&1; then \
			echo "stopping dev server (pid=$$pid)..."; \
			kill "$$pid" || true; \
			sleep 1; \
			if kill -0 "$$pid" >/dev/null 2>&1; then \
				echo "force kill dev server (pid=$$pid)..."; \
				kill -9 "$$pid" || true; \
			fi; \
		else \
			echo "no running process for pid=$$pid"; \
		fi; \
		rm -f "$(DEV_PID_FILE)"; \
	else \
		echo "no dev pid file found: $(DEV_PID_FILE)"; \
	fi
