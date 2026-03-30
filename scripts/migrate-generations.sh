#!/bin/bash

# Migration script for generation_requests user_id
# Usage: ./migrate-generations.sh [dry-run|apply]

set -e

MODE=${1:-dry-run}

if [ "$MODE" != "dry-run" ] && [ "$MODE" != "apply" ]; then
    echo "Usage: $0 [dry-run|apply]"
    echo ""
    echo "  dry-run  - Show what will be changed (default)"
    echo "  apply    - Actually apply the changes"
    exit 1
fi

DRY_RUN="true"
if [ "$MODE" = "apply" ]; then
    DRY_RUN="false"
fi

echo "Running migration in $MODE mode..."
echo ""

# Run the migration script inside the web-backend container
docker exec aifacebot_web_backend bash -c "cd /app/cmd/migrate-generations && go run main.go \
  -host=postgres \
  -port=5432 \
  -user=aifacebot_user \
  -password=aifacebot_password \
  -db=aifacebot \
  -dry-run=$DRY_RUN"

echo ""
echo "Migration completed!"
