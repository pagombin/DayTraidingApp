# Trading Platform

An intelligent, self-healing trading platform with a web dashboard.

## Quick Start

```bash
bash setup.sh
```

This runs the interactive setup wizard, starts all services, and opens the dashboard.

## Manual Start

```bash
cp .env.example .env   # Edit with your values
make up                # Start all services
```

## Services

| Service   | Port | Description                    |
|-----------|------|--------------------------------|
| Frontend  | 3000 | Next.js dashboard              |
| Gateway   | 8080 | REST API                       |
| WebSocket | 8081 | Real-time data stream          |
| Postgres  | 5432 | TimescaleDB database           |
| Redis     | 6379 | Cache and pub/sub              |

## Commands

```bash
make up          # Start all services
make down        # Stop all services
make logs        # View all logs
make health      # Check service health
make backup      # Backup database
make setup       # Run setup wizard
```
