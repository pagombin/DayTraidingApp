package config

import (
	"context"
	"encoding/json"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type Config struct {
	AlpacaAPIKey    string
	AlpacaAPISecret string
	AlpacaDataURL   string
	AlpacaRestURL   string
	DBHost          string
	DBPort          string
	DBUser          string
	DBPassword      string
	DBName          string
	RedisURL        string
	HTTPPort        string
	LogLevel        string
}

func Load() Config {
	return Config{
		AlpacaAPIKey:    os.Getenv("ALPACA_API_KEY"),
		AlpacaAPISecret: os.Getenv("ALPACA_API_SECRET"),
		AlpacaDataURL:   getEnv("ALPACA_DATA_URL", "wss://stream.data.alpaca.markets/v2/iex"),
		AlpacaRestURL:   getEnv("ALPACA_REST_URL", "https://data.alpaca.markets/v2"),
		DBHost:          getEnv("DB_HOST", "postgres"),
		DBPort:          getEnv("DB_PORT", "5432"),
		DBUser:          getEnv("DB_USER", "trading"),
		DBPassword:      getEnv("DB_PASSWORD", "changeme_in_production"),
		DBName:          getEnv("DB_NAME", "trading_platform"),
		RedisURL:        getEnv("REDIS_URL", "redis://redis:6379"),
		HTTPPort:        getEnv("HTTP_PORT", "8083"),
		LogLevel:        getEnv("LOG_LEVEL", "info"),
	}
}

func (c Config) DSN() string {
	return "postgres://" + c.DBUser + ":" + c.DBPassword + "@" + c.DBHost + ":" + c.DBPort + "/" + c.DBName + "?sslmode=disable"
}

// LoadWatchlist reads the watchlist from app_config table.
func LoadWatchlist(ctx context.Context, pool *pgxpool.Pool) ([]string, error) {
	var value json.RawMessage
	err := pool.QueryRow(ctx,
		"SELECT value FROM app_config WHERE key = 'watchlist.symbols'").Scan(&value)
	if err != nil {
		// Return default watchlist if not configured
		return []string{"SPY", "QQQ", "AAPL", "MSFT", "GOOGL", "AMZN", "NVDA", "META", "TSLA", "IWM"}, nil
	}

	var symbols []string
	if err := json.Unmarshal(value, &symbols); err != nil {
		// Try parsing as a string (e.g., "SPY,QQQ,AAPL")
		var str string
		if err2 := json.Unmarshal(value, &str); err2 == nil {
			for _, s := range strings.Split(str, ",") {
				s = strings.TrimSpace(s)
				if s != "" {
					symbols = append(symbols, strings.ToUpper(s))
				}
			}
			return symbols, nil
		}
		return nil, err
	}

	return symbols, nil
}

// SubscribeConfigChanges listens for config hot-reload events.
func SubscribeConfigChanges(ctx context.Context, rdb *redis.Client, logger *zap.Logger, onWatchlistChange func([]string)) {
	sub := rdb.Subscribe(ctx, "config:changed")
	defer sub.Close()

	ch := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-ch:
			var payload struct {
				Key   string          `json:"key"`
				Value json.RawMessage `json:"value"`
			}
			if err := json.Unmarshal([]byte(msg.Payload), &payload); err != nil {
				logger.Warn("Failed to parse config change", zap.Error(err))
				continue
			}

			if payload.Key == "watchlist.symbols" {
				var symbols []string
				if err := json.Unmarshal(payload.Value, &symbols); err != nil {
					logger.Warn("Failed to parse watchlist change", zap.Error(err))
					continue
				}
				logger.Info("Watchlist updated via hot-reload", zap.Strings("symbols", symbols))
				if onWatchlistChange != nil {
					onWatchlistChange(symbols)
				}
			}
		}
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
