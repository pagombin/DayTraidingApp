-- Enable TimescaleDB extension
CREATE EXTENSION IF NOT EXISTS timescaledb;

-- Market tick data (high-frequency time-series)
CREATE TABLE IF NOT EXISTS market_ticks (
    symbol TEXT NOT NULL,
    timestamp TIMESTAMPTZ NOT NULL,
    bid NUMERIC(12,4),
    ask NUMERIC(12,4),
    last_price NUMERIC(12,4),
    volume BIGINT,
    source TEXT
);

SELECT create_hypertable('market_ticks', 'timestamp',
    chunk_time_interval => INTERVAL '1 day',
    if_not_exists => TRUE
);

CREATE INDEX IF NOT EXISTS idx_ticks_symbol_time ON market_ticks (symbol, timestamp DESC);

-- Option quotes (time-series)
CREATE TABLE IF NOT EXISTS option_quotes (
    contract_symbol TEXT NOT NULL,
    underlying TEXT NOT NULL,
    timestamp TIMESTAMPTZ NOT NULL,
    expiration DATE,
    strike NUMERIC(10,2),
    option_type TEXT CHECK (option_type IN ('call', 'put')),
    bid NUMERIC(10,4),
    ask NUMERIC(10,4),
    last_price NUMERIC(10,4),
    volume INT,
    open_interest INT,
    implied_volatility NUMERIC(8,6),
    delta NUMERIC(8,6),
    gamma NUMERIC(8,6),
    theta NUMERIC(8,6),
    vega NUMERIC(8,6),
    rho NUMERIC(8,6)
);

SELECT create_hypertable('option_quotes', 'timestamp',
    chunk_time_interval => INTERVAL '1 day',
    if_not_exists => TRUE
);

CREATE INDEX IF NOT EXISTS idx_options_underlying_time ON option_quotes (underlying, timestamp DESC);

-- Data retention policy: auto-drop tick data older than 90 days
SELECT add_retention_policy('market_ticks', INTERVAL '90 days', if_not_exists => TRUE);
SELECT add_retention_policy('option_quotes', INTERVAL '90 days', if_not_exists => TRUE);
