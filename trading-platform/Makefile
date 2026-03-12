.PHONY: up down logs restart health backup setup

# Start all services
up:
	docker compose up -d

# Start all services and rebuild
up-build:
	docker compose up -d --build

# Stop all services
down:
	docker compose down

# View logs (all services)
logs:
	docker compose logs -f --tail=100

# View logs for a specific service
logs-%:
	docker compose logs -f --tail=100 $*

# Restart a specific service
restart-%:
	docker compose restart $*

# Check health of all services
health:
	@echo "Gateway:" && curl -sf http://localhost:8080/healthz | python3 -m json.tool 2>/dev/null || echo "  UNHEALTHY"
	@echo "Frontend:" && curl -sf http://localhost:3000 > /dev/null && echo '  {"status": "ok"}' || echo "  UNHEALTHY"
	@echo "PostgreSQL:" && docker compose exec -T postgres pg_isready -U trading > /dev/null && echo '  {"status": "ok"}' || echo "  UNHEALTHY"
	@echo "Redis:" && docker compose exec -T redis redis-cli ping > /dev/null && echo '  {"status": "ok"}' || echo "  UNHEALTHY"

# Backup database
backup:
	@mkdir -p backups
	docker compose exec -T postgres pg_dump -U trading trading_platform | gzip > backups/backup_$$(date +%Y%m%d_%H%M%S).sql.gz
	@echo "Backup saved to backups/"

# Run setup wizard
setup:
	bash setup.sh
