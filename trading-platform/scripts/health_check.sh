#!/usr/bin/env bash

echo "=== Trading Platform Health Check ==="
echo ""

echo "Gateway:"
curl -sf http://localhost:8080/healthz 2>/dev/null | python3 -m json.tool 2>/dev/null || echo "  UNREACHABLE"
echo ""

echo "Gateway Readiness:"
curl -sf http://localhost:8080/readyz 2>/dev/null | python3 -m json.tool 2>/dev/null || echo "  UNREACHABLE"
echo ""

echo "Frontend:"
curl -sf http://localhost:3000 > /dev/null 2>&1 && echo "  OK" || echo "  UNREACHABLE"
echo ""

echo "PostgreSQL:"
docker compose exec -T postgres pg_isready -U trading > /dev/null 2>&1 && echo "  OK" || echo "  UNREACHABLE"
echo ""

echo "Redis:"
docker compose exec -T redis redis-cli ping > /dev/null 2>&1 && echo "  OK" || echo "  UNREACHABLE"
