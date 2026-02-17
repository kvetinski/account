.PHONY: build test integration integration-only test-integration test-integration-only proto run run-tracing run-metrics run-observability down \
	k8s-kind-create k8s-kind-delete k8s-load-image k8s-apply k8s-delete \
	k8s-migrate k8s-pf-grafana k8s-pf-prometheus k8s-pf-jaeger k8s-pf-account k8s-pf-cadvisor \
	k8s-refresh k8s-up k8s-down-all

build:
	go build -o bin/account ./cmd/account

test:
	go test ./...

test-up-int-down:
	@set -e; \
	echo "Starting integration dependencies (db + migrate)..."; \
	docker compose up --build -d db migrate; \
	echo "Running integration tests..."; \
	if TEST_POSTGRES_URI="postgres://account:account@localhost:5432/account?sslmode=disable" go test -tags=integration ./...; then \
		echo "Integration tests passed, stopping containers..."; \
		docker compose down -v --remove-orphans; \
	else \
		echo "Integration tests failed, containers are left running for debugging."; \
		exit 1; \
	fi

test-integration:
	TEST_POSTGRES_URI="postgres://account:account@localhost:5432/account?sslmode=disable" go test -tags=integration ./...

proto:
	@PATH="$$PATH:$$HOME/go/bin" protoc \
		--go_out=. --go_opt=module=github.com/kvetinski/account \
		--go-grpc_out=. --go-grpc_opt=module=github.com/kvetinski/account \
		proto/account/v1/account.proto

run:
	docker compose up --build -d

run-tracing:
	OTEL_ENABLED=true docker compose --profile tracing up --build -d

run-metrics:
	docker compose --profile metrics up --build -d

run-observability:
	OTEL_ENABLED=true docker compose --profile tracing --profile metrics up --build -d

down:
	docker compose --profile tracing --profile metrics down -v --remove-orphans

k8s-kind-create:
	kind create cluster --name account

k8s-kind-delete:
	timeout 120s kind delete cluster --name account || true

k8s-load-image:
	docker build -t account:local .
	kind load docker-image account:local --name account

k8s-apply:
	timeout 60s kubectl --request-timeout=20s apply -k k8s

k8s-refresh:
	kubectl --request-timeout=20s apply -k k8s
	kubectl --request-timeout=20s -n account get deployment -o name | xargs -r kubectl --request-timeout=20s -n account rollout restart
	kubectl --request-timeout=20s -n account get statefulset -o name | xargs -r kubectl --request-timeout=20s -n account rollout restart || true
	kubectl --request-timeout=20s -n account get daemonset -o name | xargs -r kubectl --request-timeout=20s -n account rollout restart || true

k8s-delete:
	timeout 40s kubectl --request-timeout=20s delete -k k8s --ignore-not-found --wait=false || true

k8s-migrate:
	@set -e; \
	if ! kubectl --request-timeout=20s -n account rollout status deployment/postgres --timeout=420s; then \
		echo "postgres is not ready, collecting diagnostics..."; \
		kubectl --request-timeout=20s -n account get pods -o wide || true; \
		kubectl --request-timeout=20s -n account describe deployment/postgres || true; \
		kubectl --request-timeout=20s -n account describe pod -l app=postgres || true; \
		kubectl --request-timeout=20s -n account logs deploy/postgres --tail=200 || true; \
		exit 1; \
	fi
	kubectl --request-timeout=20s -n account delete job account-migrate --ignore-not-found
	kubectl --request-timeout=20s apply -f k8s/account/migration-job.yaml
	kubectl --request-timeout=20s -n account wait --for=condition=complete --timeout=420s job/account-migrate
	kubectl --request-timeout=20s -n account logs job/account-migrate

k8s-pf-grafana:
	kubectl -n account port-forward svc/grafana 3000:3000

k8s-pf-prometheus:
	kubectl -n account port-forward svc/prometheus 9090:9090

k8s-pf-jaeger:
	kubectl -n account port-forward svc/jaeger 16686:16686

k8s-pf-account:
	kubectl -n account port-forward svc/account 8080:8080

k8s-pf-cadvisor:
	kubectl -n account port-forward svc/cadvisor 8081:8080

k8s-pf-postgres:
	kubectl -n account port-forward svc/postgres 5432:5432

k8s-up:
	@set -e; \
	if ! kind get clusters | grep -qx "account"; then \
		$(MAKE) k8s-kind-create; \
	else \
		echo "kind cluster 'account' already exists"; \
	fi
	$(MAKE) k8s-load-image
	$(MAKE) k8s-apply
	$(MAKE) k8s-migrate

k8s-down-all:
	$(MAKE) k8s-delete
	timeout 120s kind delete cluster --name account || true
