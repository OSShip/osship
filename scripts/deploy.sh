#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

echo "==> Pulling latest changes..."
git pull origin main 2>/dev/null || true
git submodule update --init --recursive

echo "==> Building images (one service at a time)..."
SERVICES=(gateway auth users listings mentors sessions notifications payments metrics ui)
for svc in "${SERVICES[@]}"; do
  echo "    building $svc..."
  docker buildx bake "$svc"
done

echo "==> Starting infrastructure..."
docker compose up -d postgres redis kafka
sleep 5
docker compose run --rm kafka-init
make migrate

echo "==> Starting application stack..."
docker compose up -d

echo "==> Waiting for health checks..."
sleep 15
./scripts/health-check.sh || true
./scripts/smoke-test.sh || true

echo "==> Deploy complete."
