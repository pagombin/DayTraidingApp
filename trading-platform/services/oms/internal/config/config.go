package config

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

type Config struct {
	// Database
	DBHost     string
	DBPort     int
	DBUser     string
	DBPassword string
	DBName     string

	// Redis
	RedisURL string

	// Alpaca
	AlpacaAPIKey    string
	AlpacaAPISecret string
	AlpacaPaperURL  string
	AlpacaLiveURL   string
	AlpacaStreamURL string

	// Service
	HTTPPort string
	LogLevel string

	// Risk (hot-reloaded)
	Risk RiskConfig

	// System
	AutonomyLevel int
	PaperMode     bool

	mu sync.RWMutex
}

type RiskConfig struct {
	MaxPositionSizePct     decimal.Decimal
	MaxPortfolioRiskPct    decimal.Decimal
	MaxDailyLoss           decimal.Decimal
	MaxSingleTradeLoss     decimal.Decimal
	MaxDeltaExposure       decimal.Decimal
	MaxOrdersPerMinute     int
	MaxSectorConcentration decimal.Decimal
	RequireStopLoss        bool
	RestrictedSymbols      map[string]bool
	CircuitBreakerEnabled  bool
}

func Load() *Config {
	port, _ := strconv.Atoi(getEnv("DB_PORT", "5432"))
	return &Config{
		DBHost:          getEnv("DB_HOST", "postgres"),
		DBPort:          port,
		DBUser:          getEnv("DB_USER", "trading"),
		DBPassword:      getEnv("DB_PASSWORD", "changeme_in_production"),
		DBName:          getEnv("DB_NAME", "trading_platform"),
		RedisURL:        getEnv("REDIS_URL", "redis://redis:6379"),
		AlpacaAPIKey:    getEnv("ALPACA_API_KEY", ""),
		AlpacaAPISecret: getEnv("ALPACA_API_SECRET", ""),
		AlpacaPaperURL:  getEnv("ALPACA_PAPER_URL", "https://paper-api.alpaca.markets"),
		AlpacaLiveURL:   getEnv("ALPACA_LIVE_URL", "https://api.alpaca.markets"),
		AlpacaStreamURL: getEnv("ALPACA_STREAM_URL", "wss://paper-api.alpaca.markets/stream"),
		HTTPPort:        getEnv("HTTP_PORT", "8084"),
		LogLevel:        getEnv("LOG_LEVEL", "info"),
		Risk: RiskConfig{
			MaxPositionSizePct:     decimal.NewFromInt(5),
			MaxPortfolioRiskPct:    decimal.NewFromInt(15),
			MaxDailyLoss:           decimal.NewFromInt(500),
			MaxSingleTradeLoss:     decimal.NewFromInt(200),
			MaxDeltaExposure:       decimal.NewFromInt(500),
			MaxOrdersPerMinute:     10,
			MaxSectorConcentration: decimal.NewFromInt(30),
			RequireStopLoss:        true,
			RestrictedSymbols:      map[string]bool{},
			CircuitBreakerEnabled:  true,
		},
		AutonomyLevel: 1,
		PaperMode:     true,
	}
}

func (c *Config) LoadFromDB(pool *pgxpool.Pool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := pool.Query(ctx, "SELECT key, value FROM app_config WHERE category IN ('risk', 'broker', 'system')")
	if err != nil {
		return err
	}
	defer rows.Close()

	c.mu.Lock()
	defer c.mu.Unlock()

	for rows.Next() {
		var key string
		var value json.RawMessage
		if err := rows.Scan(&key, &value); err != nil {
			continue
		}
		c.applyConfigValue(key, value)
	}
	return nil
}

func (c *Config) applyConfigValue(key string, value json.RawMessage) {
	switch key {
	case "risk.max_daily_loss":
		if v, err := parseDecimal(value); err == nil {
			c.Risk.MaxDailyLoss = v
		}
	case "risk.max_position_pct":
		if v, err := parseDecimal(value); err == nil {
			c.Risk.MaxPositionSizePct = v
		}
	case "risk.max_portfolio_risk_pct":
		if v, err := parseDecimal(value); err == nil {
			c.Risk.MaxPortfolioRiskPct = v
		}
	case "risk.max_single_trade_loss":
		if v, err := parseDecimal(value); err == nil {
			c.Risk.MaxSingleTradeLoss = v
		}
	case "risk.max_delta_exposure":
		if v, err := parseDecimal(value); err == nil {
			c.Risk.MaxDeltaExposure = v
		}
	case "risk.max_orders_per_minute":
		var v int
		if json.Unmarshal(value, &v) == nil {
			c.Risk.MaxOrdersPerMinute = v
		}
	case "risk.max_sector_concentration_pct":
		if v, err := parseDecimal(value); err == nil {
			c.Risk.MaxSectorConcentration = v
		}
	case "risk.require_stop_loss":
		var v bool
		if json.Unmarshal(value, &v) == nil {
			c.Risk.RequireStopLoss = v
		}
	case "risk.restricted_symbols":
		var symbols []string
		if json.Unmarshal(value, &symbols) == nil {
			m := make(map[string]bool, len(symbols))
			for _, s := range symbols {
				m[strings.ToUpper(s)] = true
			}
			c.Risk.RestrictedSymbols = m
		}
	case "risk.circuit_breaker_enabled":
		var v bool
		if json.Unmarshal(value, &v) == nil {
			c.Risk.CircuitBreakerEnabled = v
		}
	case "broker.paper_mode":
		var v bool
		if json.Unmarshal(value, &v) == nil {
			c.PaperMode = v
		}
	case "system.autonomy_level":
		var v int
		if json.Unmarshal(value, &v) == nil {
			c.AutonomyLevel = v
		}
	}
}

func (c *Config) GetRisk() RiskConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Risk
}

func (c *Config) GetAutonomyLevel() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.AutonomyLevel
}

func (c *Config) SubscribeChanges(ctx context.Context, rdb *redis.Client, log *zap.SugaredLogger, pool *pgxpool.Pool) {
	sub := rdb.Subscribe(ctx, "config:changed")
	ch := sub.Channel()

	go func() {
		for {
			select {
			case <-ctx.Done():
				sub.Close()
				return
			case msg := <-ch:
				log.Infow("Config changed, reloading", "payload", msg.Payload)
				if err := c.LoadFromDB(pool); err != nil {
					log.Errorw("Failed to reload config", "error", err)
				}
			}
		}
	}()
}

func parseDecimal(raw json.RawMessage) (decimal.Decimal, error) {
	var f float64
	if err := json.Unmarshal(raw, &f); err != nil {
		var s string
		if err2 := json.Unmarshal(raw, &s); err2 != nil {
			return decimal.Zero, err
		}
		return decimal.NewFromString(s)
	}
	return decimal.NewFromFloat(f), nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
