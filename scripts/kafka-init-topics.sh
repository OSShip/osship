#!/usr/bin/env bash
# Create OSShip Kafka event topics (idempotent).
set -euo pipefail

BOOTSTRAP="${KAFKA_BOOTSTRAP:-kafka:9092}"
TOPICS=(
  listing.events
  enrollment.events
  payment.events
  session.events
  mentor.events
)

echo "==> Creating Kafka topics (bootstrap: $BOOTSTRAP)"

for topic in "${TOPICS[@]}"; do
  /opt/kafka/bin/kafka-topics.sh \
    --bootstrap-server "$BOOTSTRAP" \
    --create --if-not-exists \
    --topic "$topic" \
    --partitions 1 \
    --replication-factor 1
  echo "    ready: $topic"
done

echo "==> Kafka topics initialized"
