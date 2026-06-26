#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

echo "OSShip Pulling latest changes..."
git pull origin main 2>/dev/null || true

echo "OSShip Building images..."
docker buildx bake

echo "OSShip Running migrations..."
docker compose up -d postgres redis kafka
sleep 5
make migrate

echo "OSShip Starting stack..."
docker compose up -d

echo "OSShip Waiting for health checks..."
sleep 15

echo "OSShip: gateway health"
curl -sf http://localhost/api/v1/health || curl -sf http://localhost:8080/health || echo "Gateway not ready yet"

echo "OSShip Deploy complete."
