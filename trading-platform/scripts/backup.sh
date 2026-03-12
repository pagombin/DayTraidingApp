#!/usr/bin/env bash
set -euo pipefail

BACKUP_DIR="$(dirname "$0")/../backups"
mkdir -p "$BACKUP_DIR"

TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="$BACKUP_DIR/backup_${TIMESTAMP}.sql.gz"

echo "Backing up database..."
docker compose exec -T postgres pg_dump -U trading trading_platform | gzip > "$BACKUP_FILE"
echo "Backup saved to $BACKUP_FILE"
