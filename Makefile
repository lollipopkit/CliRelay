SHELL := /bin/bash

.PHONY: dev-up dev-restart dev-down

DEV_NAME ?= cli-relay-dev
DEV_PANEL_NAME ?= cli-relay-dev-panel
DEV_CMD ?= go run ./cmd/server
DEV_PID_FILE ?= .tmp/$(DEV_NAME).pid
DEV_LOG_FILE ?= .tmp/$(DEV_NAME).log
DEV_PANEL_PID_FILE ?= .tmp/$(DEV_PANEL_NAME).pid
DEV_PANEL_LOG_FILE ?= .tmp/$(DEV_PANEL_NAME).log
DEV_PANEL_DIR ?= panel
DEV_PANEL_CMD ?= bun run dev
DEV_MANAGEMENT_PANEL_URL ?= http://127.0.0.1:5173
DEV_GO_CACHE ?= $(CURDIR)/.tmp/go-build
DEV_BIN ?= .tmp/$(DEV_NAME)-server
DEV_FORCE_KILL_PORT ?= 0

dev-up:
	@set -e; \
	mkdir -p .tmp/go-build; \
	if [ -f "$(DEV_PID_FILE)" ] && kill -0 "$$(cat $(DEV_PID_FILE))" >/dev/null 2>&1; then \
		echo "dev server already running (pid=$$(cat $(DEV_PID_FILE)))"; \
		exit 1; \
	fi; \
	if [ -f "$(DEV_PANEL_PID_FILE)" ] && kill -0 "$$(cat $(DEV_PANEL_PID_FILE))" >/dev/null 2>&1; then \
		echo "dev panel already running (pid=$$(cat $(DEV_PANEL_PID_FILE)))"; \
		exit 1; \
	fi; \
	port="$$(awk '/^[[:space:]]*port:[[:space:]]*/ {print $$2; exit}' config.yaml 2>/dev/null | tr -d '\"' | tr -d ' ')"; \
	if [ -z "$$port" ]; then \
		port=8317; \
	fi; \
	if command -v lsof >/dev/null 2>&1; then \
		bound_pid="$$(lsof -nP -iTCP:$$port -sTCP:LISTEN -t 2>/dev/null | awk 'NR==1 {print $$1; exit}')"; \
		if [ -n "$$bound_pid" ]; then \
			legacy_pid=""; \
			if [ -f "$(DEV_PID_FILE)" ]; then \
				legacy_pid="$$(cat "$(DEV_PID_FILE)")"; \
			fi; \
			if [ "$$bound_pid" != "$$legacy_pid" ]; then \
				echo "port $$port is already in use (pid=$$bound_pid)"; \
				if [ "$(DEV_FORCE_KILL_PORT)" = "1" ]; then \
					echo "killing bound process $$bound_pid by DEV_FORCE_KILL_PORT=1"; \
					kill "$$bound_pid" || true; \
					sleep 1; \
					if kill -0 "$$bound_pid" >/dev/null 2>&1; then \
						kill -9 "$$bound_pid" || true; \
					fi; \
					sleep 1; \
					bound_pid_after="$$(lsof -nP -iTCP:$$port -sTCP:LISTEN -t 2>/dev/null | awk 'NR==1 {print $$1; exit}')"; \
					if [ -n "$$bound_pid_after" ]; then \
						echo "port $$port still in use (pid=$$bound_pid_after)."; \
						echo "set DEV_FORCE_KILL_PORT=1 and rerun with proper permission, or choose another port in config."; \
						exit 1; \
					fi; \
				else \
					echo "set DEV_FORCE_KILL_PORT=1 to auto-kill it, or choose another port in config."; \
					exit 1; \
				fi; \
			fi; \
		fi; \
	else \
		echo "warning: lsof not found, cannot preflight port $$port"; \
	fi; \
	echo "starting CliRelay in local dev mode..."; \
	GOCACHE="$(DEV_GO_CACHE)" go build -o "$(DEV_BIN)" ./cmd/server; \
	nohup bash -lc 'cd "$(DEV_PANEL_DIR)" && $(DEV_PANEL_CMD) -- --host 127.0.0.1 --port 5173 --strictPort & child=$$!; trap "kill $$child >/dev/null 2>&1 || true; wait $$child >/dev/null 2>&1 || true" TERM INT; wait $$child' >> "$(DEV_PANEL_LOG_FILE)" 2>&1 & \
		echo "$$!" > "$(DEV_PANEL_PID_FILE)"; \
	sleep 1; \
	nohup env GOCACHE="$(DEV_GO_CACHE)" MANAGEMENT_DEV_URL="$(DEV_MANAGEMENT_PANEL_URL)" "$(DEV_BIN)" -config "config.yaml" >> "$(DEV_LOG_FILE)" 2>&1 & \
		echo "$$!" > "$(DEV_PID_FILE)"; \
	sleep 1; \
	echo "started pid=$$(cat $(DEV_PID_FILE))"; \
	echo "started panel pid=$$(cat $(DEV_PANEL_PID_FILE))"; \
	echo "log: $(DEV_LOG_FILE)"; \
	echo "panel log: $(DEV_PANEL_LOG_FILE)"; \
	echo "management ui: $(DEV_MANAGEMENT_PANEL_URL)"

dev-restart:
	@$(MAKE) dev-down DEV_FORCE_KILL_PORT=1
	@$(MAKE) dev-up DEV_FORCE_KILL_PORT=1

dev-down:
	@set -e; \
	port="$$(awk '/^[[:space:]]*port:[[:space:]]*/ {print $$2; exit}' config.yaml 2>/dev/null | tr -d '\"' | tr -d ' ')"; \
	if [ -z "$$port" ]; then \
		port=8317; \
	fi; \
	if [ -f "$(DEV_PID_FILE)" ]; then \
		pid="$$(cat "$(DEV_PID_FILE)")"; \
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
	fi; \
	if [ "$(DEV_FORCE_KILL_PORT)" = "1" ] && command -v lsof >/dev/null 2>&1; then \
		bound_pid="$$(lsof -nP -iTCP:$$port -sTCP:LISTEN -t 2>/dev/null | awk 'NR==1 {print $$1; exit}')"; \
		if [ -n "$$bound_pid" ]; then \
			echo "killing stale dev server on port $$port (pid=$$bound_pid) by DEV_FORCE_KILL_PORT=1"; \
			kill "$$bound_pid" || true; \
			sleep 1; \
			if kill -0 "$$bound_pid" >/dev/null 2>&1; then \
				kill -9 "$$bound_pid" || true; \
			fi; \
			sleep 1; \
			bound_pid_after="$$(lsof -nP -iTCP:$$port -sTCP:LISTEN -t 2>/dev/null | awk 'NR==1 {print $$1; exit}')"; \
			if [ -n "$$bound_pid_after" ]; then \
				echo "port $$port still in use after cleanup (pid=$$bound_pid_after)."; \
				exit 1; \
			fi; \
		fi; \
	elif [ "$(DEV_FORCE_KILL_PORT)" = "1" ]; then \
		echo "warning: lsof not found, cannot check stale dev server on port $$port"; \
	fi; \
	if [ -f "$(DEV_PANEL_PID_FILE)" ]; then \
		pid="$$(cat "$(DEV_PANEL_PID_FILE)")"; \
		if kill -0 "$$pid" >/dev/null 2>&1; then \
			echo "stopping dev panel (pid=$$pid)..."; \
			kill "$$pid" || true; \
			sleep 1; \
			if kill -0 "$$pid" >/dev/null 2>&1; then \
				echo "force kill dev panel (pid=$$pid)..."; \
				kill -9 "$$pid" || true; \
			fi; \
		else \
			echo "no running panel process for pid=$$pid"; \
		fi; \
		rm -f "$(DEV_PANEL_PID_FILE)"; \
	else \
		echo "no dev panel pid file found: $(DEV_PANEL_PID_FILE)"; \
	fi
