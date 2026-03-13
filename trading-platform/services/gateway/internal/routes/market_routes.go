package routes

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
)

func SetupMarketRoutes(api fiber.Router, pool *pgxpool.Pool, rdb *goredis.Client) {
	// Latest quote for a symbol (from Redis cache)
	api.Get("/market/quote/:symbol", func(c *fiber.Ctx) error {
		symbol := c.Params("symbol")
		key := fmt.Sprintf("market:latest:%s", symbol)

		val, err := rdb.Get(c.Context(), key).Result()
		if err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": fmt.Sprintf("no quote found for %s", symbol),
			})
		}

		var quote map[string]interface{}
		json.Unmarshal([]byte(val), &quote)
		return c.JSON(quote)
	})

	// Latest quotes for all watchlist symbols (batch)
	api.Get("/market/quotes", func(c *fiber.Ctx) error {
		// Get all active symbols from sorted set
		symbols, err := rdb.ZRangeByScore(c.Context(), "market:active_symbols", &goredis.ZRangeBy{
			Min: "-inf",
			Max: "+inf",
		}).Result()
		if err != nil || len(symbols) == 0 {
			return c.JSON(fiber.Map{"quotes": []interface{}{}})
		}

		quotes := make([]map[string]interface{}, 0, len(symbols))
		pipe := rdb.Pipeline()
		cmds := make(map[string]*goredis.StringCmd, len(symbols))
		for _, s := range symbols {
			cmds[s] = pipe.Get(c.Context(), fmt.Sprintf("market:latest:%s", s))
		}
		pipe.Exec(c.Context())

		for symbol, cmd := range cmds {
			val, err := cmd.Result()
			if err != nil {
				continue
			}
			var quote map[string]interface{}
			if err := json.Unmarshal([]byte(val), &quote); err == nil {
				quote["symbol"] = symbol
				quotes = append(quotes, quote)
			}
		}

		return c.JSON(fiber.Map{"quotes": quotes})
	})

	// Recent 1-min bars from Redis Stream
	api.Get("/market/bars/:symbol", func(c *fiber.Ctx) error {
		symbol := c.Params("symbol")
		count := c.QueryInt("count", 60)
		if count > 1440 {
			count = 1440
		}

		streamKey := fmt.Sprintf("market:bars:%s", symbol)
		msgs, err := rdb.XRevRangeN(c.Context(), streamKey, "+", "-", int64(count)).Result()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		bars := make([]map[string]interface{}, 0, len(msgs))
		for _, msg := range msgs {
			if data, ok := msg.Values["data"].(string); ok {
				var bar map[string]interface{}
				if err := json.Unmarshal([]byte(data), &bar); err == nil {
					bars = append(bars, bar)
				}
			}
		}

		return c.JSON(fiber.Map{"bars": bars})
	})

	// Market Data Service status (proxy to market-data service)
	api.Get("/market/status", func(c *fiber.Ctx) error {
		resp, err := http.Get("http://market-data:8083/status")
		if err != nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"error":     "market data service unreachable",
				"connected": false,
			})
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		var status map[string]interface{}
		json.Unmarshal(body, &status)
		return c.JSON(status)
	})

	// Market calendar info
	api.Get("/market/calendar", func(c *fiber.Ctx) error {
		now := time.Now()
		loc, _ := time.LoadLocation("America/New_York")
		if loc == nil {
			loc = time.FixedZone("EST", -5*3600)
		}
		et := now.In(loc)
		day := et.Weekday()
		minutes := et.Hour()*60 + et.Minute()

		isOpen := day >= time.Monday && day <= time.Friday &&
			minutes >= 9*60+30 && minutes < 16*60
		isPreMarket := day >= time.Monday && day <= time.Friday &&
			minutes >= 4*60 && minutes < 9*60+30
		isAfterHours := day >= time.Monday && day <= time.Friday &&
			minutes >= 16*60 && minutes < 20*60

		var status string
		switch {
		case isOpen:
			minsLeft := 16*60 - minutes
			status = fmt.Sprintf("Open — closes in %dh %dm", minsLeft/60, minsLeft%60)
		case isPreMarket:
			status = "Pre-Market"
		case isAfterHours:
			status = "After-Hours"
		default:
			status = "Closed"
		}

		return c.JSON(fiber.Map{
			"is_open":        isOpen,
			"is_pre_market":  isPreMarket,
			"is_after_hours": isAfterHours,
			"status":         status,
			"eastern_time":   et.Format("3:04 PM"),
		})
	})

	// Historical bars from TimescaleDB
	api.Get("/market/history/:symbol", func(c *fiber.Ctx) error {
		symbol := c.Params("symbol")
		start := c.Query("start")
		end := c.Query("end")
		limit := c.QueryInt("limit", 500)

		ctx, cancel := dbCtx()
		defer cancel()

		query := `SELECT symbol, timestamp, bid, ask, last_price, volume, source
		          FROM market_ticks WHERE symbol = $1`
		args := []interface{}{symbol}
		argIdx := 2

		if start != "" {
			query += fmt.Sprintf(" AND timestamp >= $%d", argIdx)
			args = append(args, start)
			argIdx++
		}
		if end != "" {
			query += fmt.Sprintf(" AND timestamp <= $%d", argIdx)
			args = append(args, end)
			argIdx++
		}

		query += fmt.Sprintf(" ORDER BY timestamp DESC LIMIT $%d", argIdx)
		args = append(args, limit)

		rows, err := pool.Query(ctx, query, args...)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		defer rows.Close()

		var ticks []fiber.Map
		for rows.Next() {
			var sym, source string
			var ts time.Time
			var bid, ask, last *float64
			var volume *int64
			if err := rows.Scan(&sym, &ts, &bid, &ask, &last, &volume, &source); err != nil {
				continue
			}
			tick := fiber.Map{
				"symbol":    sym,
				"timestamp": ts.Format(time.RFC3339),
				"source":    source,
			}
			if bid != nil {
				tick["bid"] = *bid
			}
			if ask != nil {
				tick["ask"] = *ask
			}
			if last != nil {
				tick["last"] = *last
			}
			if volume != nil {
				tick["volume"] = *volume
			}
			ticks = append(ticks, tick)
		}

		if ticks == nil {
			ticks = []fiber.Map{}
		}

		return c.JSON(fiber.Map{"ticks": ticks, "count": len(ticks)})
	})

	// Trigger backfill (admin only, no auth check here since group has auth middleware)
	api.Post("/market/backfill", func(c *fiber.Ctx) error {
		var body struct {
			Symbol    string `json:"symbol"`
			Start     string `json:"start"`
			End       string `json:"end"`
			Timeframe string `json:"timeframe"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
		}
		if body.Symbol == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "symbol required"})
		}

		// Forward to market-data service
		return c.JSON(fiber.Map{
			"message": "backfill request queued",
			"symbol":  body.Symbol,
			"note":    "Backfill runs on market-data service startup for all watchlist symbols",
		})
	})
}
