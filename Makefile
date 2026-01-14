.PHONY: test run build lint build-node-sdk e2e-test start-scheduler-bg stop-scheduler clean-e2e

# Go targets
test:
	go test -v --race ./...

run:
	go run main.go

build:
	go build -o bin/scheduler main.go
	chmod +x bin/scheduler

lint:
	golangci-lint run --fix

# Node SDK targets
build-node-sdk:
	cd clients/node && yarn install && yarn build

# E2E testing targets
start-scheduler-bg:
	@echo "Starting scheduler in background..."
	@./bin/scheduler & echo $$! > .scheduler.pid
	@echo "Waiting for scheduler to be ready..."
	@attempt=1; \
	while [ $$attempt -le 30 ]; do \
		status=$$(curl -s -o /dev/null -w "%{http_code}" -H "api-key: $$API_KEY" http://localhost:8080/health 2>/dev/null || echo "000"); \
		if [ "$$status" = "200" ]; then \
			echo "✓ Scheduler is ready (status: $$status)"; \
			exit 0; \
		fi; \
		echo "Waiting for scheduler... ($$attempt/30) [status: $$status]"; \
		sleep 2; \
		attempt=$$((attempt + 1)); \
	done; \
	echo "✗ Scheduler failed to start"; \
	exit 1

stop-scheduler:
	@if [ -f .scheduler.pid ]; then \
		echo "Stopping scheduler (PID: $$(cat .scheduler.pid))..."; \
		kill $$(cat .scheduler.pid) 2>/dev/null || true; \
		rm .scheduler.pid; \
	fi

e2e-test: build build-node-sdk start-scheduler-bg
	@echo "Running E2E tests..."
	cd e2e && yarn install && yarn test
	@$(MAKE) stop-scheduler

clean-e2e:
	@$(MAKE) stop-scheduler
	@rm -f .scheduler.pid