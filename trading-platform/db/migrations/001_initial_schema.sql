-- ============================================================
-- CORE CONFIGURATION TABLES
-- ============================================================

-- Application configuration (dashboard-editable, hot-reloadable)
CREATE TABLE IF NOT EXISTS app_config (
    key TEXT PRIMARY KEY,
    value JSONB NOT NULL,
    category TEXT NOT NULL,
    label TEXT NOT NULL,
    description TEXT,
    value_type TEXT NOT NULL,
    constraints JSONB,
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    updated_by TEXT DEFAULT 'system'
);

-- Audit log — APPEND ONLY. No UPDATE or DELETE.
CREATE TABLE IF NOT EXISTS audit_log (
    id BIGSERIAL PRIMARY KEY,
    action TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id TEXT,
    details JSONB NOT NULL,
    user_id TEXT DEFAULT 'system',
    timestamp TIMESTAMPTZ DEFAULT NOW()
);

-- Service health records (written by Watchdog)
CREATE TABLE IF NOT EXISTS service_health (
    service_name TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('healthy', 'degraded', 'unhealthy', 'unknown')),
    last_check TIMESTAMPTZ NOT NULL,
    response_time_ms INT,
    details JSONB,
    PRIMARY KEY (service_name)
);

-- Registered services for watchdog monitoring
CREATE TABLE IF NOT EXISTS registered_services (
    name TEXT PRIMARY KEY,
    url TEXT,
    check_type TEXT NOT NULL DEFAULT 'http',
    host TEXT,
    port INT,
    critical BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Insert default registered services
INSERT INTO registered_services (name, url, check_type, host, port, critical) VALUES
    ('gateway', 'http://gateway:8080/healthz', 'http', 'gateway', 8080, true),
    ('postgres', NULL, 'tcp', 'postgres', 5432, true),
    ('redis', NULL, 'tcp', 'redis', 6379, true),
    ('frontend', 'http://frontend:3000', 'http', 'frontend', 3000, false)
ON CONFLICT (name) DO NOTHING;

-- Users (for JWT auth — dashboard access)
CREATE TABLE IF NOT EXISTS users (
    user_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'trader' CHECK (role IN ('admin', 'trader', 'viewer')),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    last_login TIMESTAMPTZ
);

-- Orders table (placeholder — will be fully built in Module 3)
CREATE TABLE IF NOT EXISTS orders (
    order_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    strategy_id TEXT NOT NULL,
    symbol TEXT NOT NULL,
    direction TEXT NOT NULL,
    order_type TEXT NOT NULL,
    quantity INT NOT NULL,
    limit_price NUMERIC(12,4),
    stop_price NUMERIC(12,4),
    status TEXT NOT NULL DEFAULT 'PENDING',
    broker_order_id TEXT,
    filled_quantity INT DEFAULT 0,
    avg_fill_price NUMERIC(12,4),
    commission NUMERIC(10,4) DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    metadata JSONB DEFAULT '{}'
);

-- Positions table (placeholder — will be fully built in Module 3)
CREATE TABLE IF NOT EXISTS positions (
    position_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id TEXT NOT NULL DEFAULT 'default',
    strategy_id TEXT NOT NULL,
    symbol TEXT NOT NULL,
    quantity INT NOT NULL,
    avg_cost NUMERIC(12,4) NOT NULL,
    opened_at TIMESTAMPTZ DEFAULT NOW(),
    is_option BOOLEAN DEFAULT FALSE,
    expiration DATE,
    strike NUMERIC(10,2),
    option_type TEXT
);

-- Events table (placeholder — will be fully built in Module 4)
CREATE TABLE IF NOT EXISTS events (
    event_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type TEXT NOT NULL,
    source TEXT NOT NULL,
    headline TEXT,
    symbols TEXT[],
    sentiment_score NUMERIC(4,3),
    impact_score INT,
    timestamp TIMESTAMPTZ NOT NULL,
    raw_data JSONB,
    processed_at TIMESTAMPTZ DEFAULT NOW()
);

-- Strategy checkpoints (for self-healing restart)
CREATE TABLE IF NOT EXISTS strategy_checkpoints (
    strategy_id TEXT NOT NULL,
    checkpoint_time TIMESTAMPTZ NOT NULL,
    state JSONB NOT NULL,
    PRIMARY KEY (strategy_id, checkpoint_time)
);
