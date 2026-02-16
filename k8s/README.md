# Local Kubernetes (kind) Guide

## 1. Create cluster
```bash
kind create cluster --name account
```

## 2. Build and load app image
```bash
docker build -t account:local .
kind load docker-image account:local --name account
```

## 3. Apply infrastructure
```bash
kubectl apply -k k8s
```

Or by namespace group:
```bash
kubectl apply -k k8s/account
kubectl apply -k k8s/observability
```

## 4. Run migration job
```bash
kubectl -n account delete job account-migrate --ignore-not-found
kubectl apply -f k8s/account/migration-job.yaml
kubectl -n account logs -f job/account-migrate
```

## 5. Verify
```bash
kubectl get pods -n account
```

## 6. Port-forward UIs
```bash
kubectl -n account port-forward svc/grafana 3000:3000
kubectl -n account port-forward svc/prometheus 9090:9090
kubectl -n account port-forward svc/jaeger 16686:16686
```

## 7. Call gRPC service locally
```bash
kubectl -n account port-forward svc/account 8080:8080
```

Then use `grpcurl` against `localhost:8080`.
