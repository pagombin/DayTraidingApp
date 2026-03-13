-- ============================================================
-- MODULE 3: Order Management Service Schema Extensions
-- ============================================================

-- Extend orders table with full OMS fields
ALTER TABLE orders ADD COLUMN IF NOT EXISTS account_id TEXT NOT NULL DEFAULT 'default';
ALTER TABLE orders ADD COLUMN IF NOT EXISTS side TEXT;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS time_in_force TEXT NOT NULL DEFAULT 'day';
ALTER TABLE orders ADD COLUMN IF NOT EXISTS trail_percent NUMERIC(8,4);
ALTER TABLE orders ADD COLUMN IF NOT EXISTS signal_id TEXT;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS rejection_reason TEXT;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS submitted_at TIMESTAMPTZ;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS filled_at TIMESTAMPTZ;

-- Migrate direction column to side column
UPDATE orders SET side = LOWER(direction) WHERE side IS NULL;
ALTER TABLE orders ALTER COLUMN side SET NOT NULL;

-- Add indexes for order queries
CREATE INDEX IF NOT EXISTS idx_orders_status ON orders(status);
CREATE INDEX IF NOT EXISTS idx_orders_symbol ON orders(symbol);
CREATE INDEX IF NOT EXISTS idx_orders_strategy ON orders(strategy_id);
CREATE INDEX IF NOT EXISTS idx_orders_created ON orders(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_orders_broker_id ON orders(broker_order_id);

-- Extend positions table
ALTER TABLE positions ADD COLUMN IF NOT EXISTS current_price NUMERIC(12,4);
ALTER TABLE positions ADD COLUMN IF NOT EXISTS market_value NUMERIC(14,4);
ALTER TABLE positions ADD COLUMN IF NOT EXISTS unrealized_pnl NUMERIC(14,4);
ALTER TABLE positions ADD COLUMN IF NOT EXISTS sector TEXT;
ALTER TABLE positions ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ DEFAULT NOW();

CREATE INDEX IF NOT EXISTS idx_positions_symbol ON positions(symbol);
CREATE INDEX IF NOT EXISTS idx_positions_account ON positions(account_id);

-- Trade journal for realized PnL tracking
CREATE TABLE IF NOT EXISTS trade_journal (
    id BIGSERIAL PRIMARY KEY,
    order_id UUID REFERENCES orders(order_id),
    symbol TEXT NOT NULL,
    side TEXT NOT NULL,
    quantity INT NOT NULL,
    entry_price NUMERIC(12,4) NOT NULL,
    exit_price NUMERIC(12,4),
    realized_pnl NUMERIC(14,4),
    commission NUMERIC(10,4) DEFAULT 0,
    strategy_id TEXT NOT NULL,
    opened_at TIMESTAMPTZ NOT NULL,
    closed_at TIMESTAMPTZ,
    hold_duration_minutes INT
);

CREATE INDEX IF NOT EXISTS idx_trade_journal_symbol ON trade_journal(symbol);
CREATE INDEX IF NOT EXISTS idx_trade_journal_closed ON trade_journal(closed_at DESC);
CREATE INDEX IF NOT EXISTS idx_trade_journal_strategy ON trade_journal(strategy_id);

-- Order state transitions log (write-ahead log)
CREATE TABLE IF NOT EXISTS order_state_log (
    id BIGSERIAL PRIMARY KEY,
    order_id UUID NOT NULL REFERENCES orders(order_id),
    old_state TEXT NOT NULL,
    new_state TEXT NOT NULL,
    reason TEXT,
    timestamp TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_order_state_log_order ON order_state_log(order_id);

-- Risk check results for each order
CREATE TABLE IF NOT EXISTS risk_check_results (
    id BIGSERIAL PRIMARY KEY,
    order_id UUID NOT NULL REFERENCES orders(order_id),
    check_name TEXT NOT NULL,
    passed BOOLEAN NOT NULL,
    reason TEXT,
    details JSONB,
    checked_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_risk_checks_order ON risk_check_results(order_id);

-- Daily portfolio snapshots for equity curve
CREATE TABLE IF NOT EXISTS portfolio_snapshots (
    id BIGSERIAL PRIMARY KEY,
    account_id TEXT NOT NULL DEFAULT 'default',
    total_equity NUMERIC(14,4) NOT NULL,
    cash NUMERIC(14,4) NOT NULL,
    buying_power NUMERIC(14,4) NOT NULL,
    unrealized_pnl NUMERIC(14,4) NOT NULL DEFAULT 0,
    realized_pnl NUMERIC(14,4) NOT NULL DEFAULT 0,
    daily_pnl NUMERIC(14,4) NOT NULL DEFAULT 0,
    position_count INT NOT NULL DEFAULT 0,
    long_exposure NUMERIC(14,4) DEFAULT 0,
    short_exposure NUMERIC(14,4) DEFAULT 0,
    snapshot_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_portfolio_snapshots_time ON portfolio_snapshots(snapshot_at DESC);

-- Day trade tracking for PDT rule
CREATE TABLE IF NOT EXISTS day_trades (
    id BIGSERIAL PRIMARY KEY,
    symbol TEXT NOT NULL,
    buy_order_id UUID,
    sell_order_id UUID,
    quantity INT NOT NULL,
    buy_price NUMERIC(12,4) NOT NULL,
    sell_price NUMERIC(12,4) NOT NULL,
    pnl NUMERIC(14,4),
    trade_date DATE NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_day_trades_date ON day_trades(trade_date DESC);

-- Add OMS config defaults
INSERT INTO app_config (key, value, category, label, description, value_type, constraints) VALUES
('system.autonomy_level', '1', 'system', 'Autonomy Level', 'Controls how much the system trades on its own (0=Observer, 1=Advisor, 2=Semi-Auto, 3=Autonomous)', 'number',
 '{"min": 0, "max": 4, "step": 1}'),
('risk.max_single_trade_loss', '200', 'risk', 'Max Single Trade Loss ($)', 'Maximum allowed loss on any single trade', 'number',
 '{"min": 25, "max": 10000, "step": 25}'),
('risk.max_delta_exposure', '500', 'risk', 'Max Delta Exposure', 'Maximum net portfolio delta across all positions', 'number',
 '{"min": 50, "max": 10000, "step": 50}'),
('risk.max_sector_concentration_pct', '30', 'risk', 'Max Sector Concentration (%)', 'Maximum percentage of portfolio in any single sector', 'number',
 '{"min": 10, "max": 100, "step": 5}'),
('risk.restricted_symbols', '[]', 'risk', 'Restricted Symbols', 'Symbols that cannot be traded', 'json', NULL),
('risk.circuit_breaker_enabled', 'true', 'risk', 'Circuit Breaker', 'Automatically halt trading when daily loss limits are approached', 'boolean', NULL)
ON CONFLICT (key) DO NOTHING;

-- Register OMS service with watchdog
INSERT INTO registered_services (name, url, check_type, host, port, critical) VALUES
    ('oms', 'http://oms:8084/healthz', 'http', 'oms', 8084, true)
ON CONFLICT (name) DO NOTHING;
