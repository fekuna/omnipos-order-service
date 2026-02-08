#!/bin/sh
# Development entrypoint

echo "Starting order-service with Air..."
cd /app/omnipos-order-service
exec air -c .air.toml
