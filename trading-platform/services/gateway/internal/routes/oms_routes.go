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

const omsBaseURL = "http://oms:8084"

func SetupOMSRoutes(api fiber.Router, pool *pgxpool.Pool, rdb *goredis.Client) {

	// ==================== ORDERS ====================

	// List orders
	api.Get("/orders", func(c *fiber.Ctx) error {
		status := c.Query("status")
		limit := c.QueryInt("limit", 50)

		ctx, cancel := dbCtx()
		defer cancel()

		query := `SELECT order_id, strategy_id, account_id, symbol, side, order_type, quantity,
			limit_price, stop_price, status, broker_order_id, filled_quantity, avg_fill_price,
			commission, signal_id, rejection_reason, time_in_force, created_at, updated_at,
			submitted_at, filled_at FROM orders`
		args := []interface{}{}

		if status != "" {
			query += " WHERE status = $1 ORDER BY created_at DESC LIMIT $2"
			args = append(args, status, limit)
		} else {
			query += " ORDER BY created_at DESC LIMIT $1"
			args = append(args, limit)
		}

		rows, err := pool.Query(ctx, query, args...)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		defer rows.Close()

		var orders []fiber.Map
		for rows.Next() {
			var orderID, strategyID, accountID, symbol, side, orderType, state, tif string
			var quantity, filledQty int
			var limitPrice, stopPrice, avgFillPrice, commission *float64
			var brokerOrderID, signalID, rejectionReason *string
			var createdAt, updatedAt time.Time
			var submittedAt, filledAt *time.Time

			if err := rows.Scan(&orderID, &strategyID, &accountID, &symbol, &side, &orderType, &quantity,
				&limitPrice, &stopPrice, &state, &brokerOrderID, &filledQty, &avgFillPrice,
				&commission, &signalID, &rejectionReason, &tif, &createdAt, &updatedAt,
				&submittedAt, &filledAt); err != nil {
				continue
			}

			order := fiber.Map{
				"order_id":     orderID,
				"strategy_id":  strategyID,
				"symbol":       symbol,
				"side":         side,
				"order_type":   orderType,
				"quantity":     quantity,
				"state":        state,
				"filled_qty":   filledQty,
				"time_in_force": tif,
				"created_at":   createdAt.Format(time.RFC3339),
				"updated_at":   updatedAt.Format(time.RFC3339),
			}
			if limitPrice != nil {
				order["limit_price"] = *limitPrice
			}
			if stopPrice != nil {
				order["stop_price"] = *stopPrice
			}
			if avgFillPrice != nil {
				order["avg_fill_price"] = *avgFillPrice
			}
			if commission != nil {
				order["commission"] = *commission
			}
			if brokerOrderID != nil {
				order["broker_order_id"] = *brokerOrderID
			}
			if rejectionReason != nil {
				order["rejection_reason"] = *rejectionReason
			}
			if submittedAt != nil {
				order["submitted_at"] = submittedAt.Format(time.RFC3339)
			}
			if filledAt != nil {
				order["filled_at"] = filledAt.Format(time.RFC3339)
			}

			orders = append(orders, order)
		}

		if orders == nil {
			orders = []fiber.Map{}
		}

		return c.JSON(fiber.Map{"orders": orders, "count": len(orders)})
	})

	// Get single order
	api.Get("/orders/:id", func(c *fiber.Ctx) error {
		orderID := c.Params("id")
		ctx, cancel := dbCtx()
		defer cancel()

		var strategyID, symbol, side, orderType, state, tif string
		var quantity, filledQty int
		var limitPrice, stopPrice, avgFillPrice, commission *float64
		var brokerOrderID, rejectionReason *string
		var createdAt, updatedAt time.Time
		var submittedAt, filledAt *time.Time

		err := pool.QueryRow(ctx,
			`SELECT strategy_id, symbol, side, order_type, quantity, limit_price, stop_price,
			 status, broker_order_id, filled_quantity, avg_fill_price, commission,
			 rejection_reason, time_in_force, created_at, updated_at, submitted_at, filled_at
			 FROM orders WHERE order_id = $1`, orderID).
			Scan(&strategyID, &symbol, &side, &orderType, &quantity, &limitPrice, &stopPrice,
				&state, &brokerOrderID, &filledQty, &avgFillPrice, &commission,
				&rejectionReason, &tif, &createdAt, &updatedAt, &submittedAt, &filledAt)
		if err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "order not found"})
		}

		order := fiber.Map{
			"order_id":     orderID,
			"strategy_id":  strategyID,
			"symbol":       symbol,
			"side":         side,
			"order_type":   orderType,
			"quantity":     quantity,
			"state":        state,
			"filled_qty":   filledQty,
			"time_in_force": tif,
			"created_at":   createdAt.Format(time.RFC3339),
			"updated_at":   updatedAt.Format(time.RFC3339),
		}
		if limitPrice != nil { order["limit_price"] = *limitPrice }
		if avgFillPrice != nil { order["avg_fill_price"] = *avgFillPrice }
		if commission != nil { order["commission"] = *commission }
		if rejectionReason != nil { order["rejection_reason"] = *rejectionReason }

		return c.JSON(order)
	})

	// Submit manual order — proxy to OMS signal system via Redis
	api.Post("/orders", func(c *fiber.Ctx) error {
		var body struct {
			Symbol      string  `json:"symbol"`
			Side        string  `json:"side"`
			OrderType   string  `json:"order_type"`
			Quantity    int     `json:"quantity"`
			LimitPrice  *string `json:"limit_price,omitempty"`
			StopPrice   *string `json:"stop_price,omitempty"`
			TimeInForce string  `json:"time_in_force,omitempty"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
		}
		if body.Symbol == "" || body.Side == "" || body.Quantity <= 0 {
			return c.Status(400).JSON(fiber.Map{"error": "symbol, side, and quantity are required"})
		}
		if body.OrderType == "" {
			body.OrderType = "market"
		}

		signal := map[string]interface{}{
			"signal_id":   fmt.Sprintf("manual-%d", time.Now().UnixNano()),
			"strategy_id": "manual",
			"symbol":      body.Symbol,
			"side":        body.Side,
			"order_type":  body.OrderType,
			"quantity":    body.Quantity,
			"timestamp":  time.Now().Format(time.RFC3339),
		}
		if body.LimitPrice != nil {
			signal["limit_price"] = *body.LimitPrice
		}
		if body.StopPrice != nil {
			signal["stop_price"] = *body.StopPrice
		}

		signalJSON, _ := json.Marshal(signal)
		ctx, cancel := dbCtx()
		defer cancel()
		rdb.Publish(ctx, "strategy:signals", string(signalJSON))

		return c.Status(201).JSON(fiber.Map{
			"message": "Order submitted",
			"signal":  signal,
		})
	})

	// Cancel order
	api.Put("/orders/:id/cancel", func(c *fiber.Ctx) error {
		orderID := c.Params("id")
		ctx, cancel := dbCtx()
		defer cancel()

		result, err := pool.Exec(ctx,
			"UPDATE orders SET status='CANCELLED', updated_at=NOW() WHERE order_id=$1 AND status NOT IN ('FILLED','CANCELLED','REJECTED','EXPIRED','FAILED')",
			orderID)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		if result.RowsAffected() == 0 {
			return c.Status(404).JSON(fiber.Map{"error": "order not found or already in terminal state"})
		}

		// Publish cancel event
		data, _ := json.Marshal(map[string]interface{}{
			"type":     "order_update",
			"order_id": orderID,
			"new_state": "CANCELLED",
		})
		rdb.Publish(ctx, "oms:order_updates", string(data))

		return c.JSON(fiber.Map{"message": "order cancelled"})
	})

	// Approve order (Autonomy Level 1)
	api.Put("/orders/:id/approve", func(c *fiber.Ctx) error {
		orderID := c.Params("id")
		ctx, cancel := dbCtx()
		defer cancel()

		result, err := pool.Exec(ctx,
			"UPDATE orders SET status='APPROVED', updated_at=NOW() WHERE order_id=$1 AND status='AWAITING_APPROVAL'",
			orderID)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		if result.RowsAffected() == 0 {
			return c.Status(404).JSON(fiber.Map{"error": "order not found or not awaiting approval"})
		}

		// Publish approval to OMS
		data, _ := json.Marshal(map[string]interface{}{
			"type":     "order_approved",
			"order_id": orderID,
		})
		rdb.Publish(ctx, "oms:order_approvals", string(data))

		return c.JSON(fiber.Map{"message": "order approved"})
	})

	// Reject order (Autonomy Level 1)
	api.Put("/orders/:id/reject", func(c *fiber.Ctx) error {
		orderID := c.Params("id")
		var body struct {
			Reason string `json:"reason"`
		}
		c.BodyParser(&body)

		ctx, cancel := dbCtx()
		defer cancel()

		result, err := pool.Exec(ctx,
			"UPDATE orders SET status='REJECTED', rejection_reason=$1, updated_at=NOW() WHERE order_id=$2 AND status='AWAITING_APPROVAL'",
			body.Reason, orderID)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		if result.RowsAffected() == 0 {
			return c.Status(404).JSON(fiber.Map{"error": "order not found or not awaiting approval"})
		}

		return c.JSON(fiber.Map{"message": "order rejected"})
	})

	// ==================== POSITIONS ====================

	api.Get("/positions", func(c *fiber.Ctx) error {
		ctx, cancel := dbCtx()
		defer cancel()

		rows, err := pool.Query(ctx,
			`SELECT position_id, account_id, strategy_id, symbol, quantity, avg_cost,
			 COALESCE(current_price, avg_cost), COALESCE(market_value, 0),
			 COALESCE(unrealized_pnl, 0), opened_at
			 FROM positions WHERE account_id = 'default' ORDER BY symbol`)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		defer rows.Close()

		var positions []fiber.Map
		for rows.Next() {
			var posID, accountID, strategyID, symbol string
			var quantity int
			var avgCost, currentPrice, marketValue, unrealizedPnL float64
			var openedAt time.Time

			if err := rows.Scan(&posID, &accountID, &strategyID, &symbol, &quantity,
				&avgCost, &currentPrice, &marketValue, &unrealizedPnL, &openedAt); err != nil {
				continue
			}

			side := "long"
			if quantity < 0 {
				side = "short"
			}

			positions = append(positions, fiber.Map{
				"position_id":   posID,
				"symbol":        symbol,
				"quantity":      quantity,
				"avg_cost":      fmt.Sprintf("%.2f", avgCost),
				"current_price": fmt.Sprintf("%.2f", currentPrice),
				"market_value":  fmt.Sprintf("%.2f", marketValue),
				"unrealized_pnl": fmt.Sprintf("%.2f", unrealizedPnL),
				"side":          side,
				"strategy_id":   strategyID,
				"opened_at":     openedAt.Format(time.RFC3339),
			})
		}

		if positions == nil {
			positions = []fiber.Map{}
		}

		return c.JSON(fiber.Map{"positions": positions})
	})

	// ==================== PORTFOLIO ====================

	api.Get("/portfolio", func(c *fiber.Ctx) error {
		// Try to get cached snapshot from Redis
		ctx, cancel := dbCtx()
		defer cancel()

		val, err := rdb.Get(ctx, "portfolio:latest").Result()
		if err == nil {
			var snapshot map[string]interface{}
			if json.Unmarshal([]byte(val), &snapshot) == nil {
				return c.JSON(snapshot)
			}
		}

		// Fallback: proxy to OMS
		return proxyToOMS(c, "GET", "/status")
	})

	api.Get("/portfolio/history", func(c *fiber.Ctx) error {
		days := c.QueryInt("days", 30)
		ctx, cancel := dbCtx()
		defer cancel()

		rows, err := pool.Query(ctx,
			`SELECT total_equity, cash, daily_pnl, position_count, snapshot_at
			 FROM portfolio_snapshots WHERE account_id = 'default'
			 ORDER BY snapshot_at DESC LIMIT $1`, days)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		defer rows.Close()

		var history []fiber.Map
		for rows.Next() {
			var equity, cash, dailyPnL float64
			var posCount int
			var snapshotAt time.Time
			if err := rows.Scan(&equity, &cash, &dailyPnL, &posCount, &snapshotAt); err != nil {
				continue
			}
			history = append(history, fiber.Map{
				"total_equity":   fmt.Sprintf("%.2f", equity),
				"cash":           fmt.Sprintf("%.2f", cash),
				"daily_pnl":      fmt.Sprintf("%.2f", dailyPnL),
				"position_count": posCount,
				"date":           snapshotAt.Format("2006-01-02"),
			})
		}

		if history == nil {
			history = []fiber.Map{}
		}

		return c.JSON(fiber.Map{"history": history})
	})

	api.Get("/portfolio/pnl", func(c *fiber.Ctx) error {
		ctx, cancel := dbCtx()
		defer cancel()

		rows, err := pool.Query(ctx,
			`SELECT strategy_id, symbol, SUM(realized_pnl) as total_pnl, COUNT(*) as trade_count
			 FROM trade_journal WHERE closed_at >= CURRENT_DATE
			 GROUP BY strategy_id, symbol ORDER BY total_pnl DESC`)
		if err != nil {
			return c.JSON(fiber.Map{"breakdown": []fiber.Map{}})
		}
		defer rows.Close()

		var breakdown []fiber.Map
		for rows.Next() {
			var strategyID, symbol string
			var totalPnL float64
			var tradeCount int
			if err := rows.Scan(&strategyID, &symbol, &totalPnL, &tradeCount); err != nil {
				continue
			}
			breakdown = append(breakdown, fiber.Map{
				"strategy_id": strategyID,
				"symbol":      symbol,
				"total_pnl":   fmt.Sprintf("%.2f", totalPnL),
				"trade_count": tradeCount,
			})
		}

		if breakdown == nil {
			breakdown = []fiber.Map{}
		}

		return c.JSON(fiber.Map{"breakdown": breakdown})
	})

	// ==================== RISK ====================

	api.Get("/risk/status", func(c *fiber.Ctx) error {
		ctx, cancel := dbCtx()
		defer cancel()

		// Defaults
		var totalEquity, dailyPnL, netDelta float64
		var maxDailyLoss, maxPositionPct, maxDeltaExposure, maxSectorConcentration float64
		var circuitBreakerEnabled bool
		totalEquity = 100000
		maxDailyLoss = 500
		maxPositionPct = 5
		maxDeltaExposure = 500
		maxSectorConcentration = 30
		circuitBreakerEnabled = true

		// Load risk config from DB
		cfgRows, cfgErr := pool.Query(ctx, "SELECT key, value FROM app_config WHERE category = 'risk'")
		if cfgErr == nil {
			defer cfgRows.Close()
			for cfgRows.Next() {
				var key, value string
				if cfgRows.Scan(&key, &value) != nil {
					continue
				}
				switch key {
				case "risk.max_daily_loss":
					fmt.Sscanf(value, "%f", &maxDailyLoss)
				case "risk.max_position_pct":
					fmt.Sscanf(value, "%f", &maxPositionPct)
				case "risk.max_delta_exposure":
					fmt.Sscanf(value, "%f", &maxDeltaExposure)
				case "risk.max_sector_concentration_pct":
					fmt.Sscanf(value, "%f", &maxSectorConcentration)
				case "risk.circuit_breaker_enabled":
					circuitBreakerEnabled = value == "true"
				}
			}
		}

		// Get portfolio snapshot from Redis
		val, redisErr := rdb.Get(ctx, "portfolio:latest").Result()
		if redisErr == nil {
			var snapshot map[string]interface{}
			if json.Unmarshal([]byte(val), &snapshot) == nil {
				if eq, ok := snapshot["total_equity"].(string); ok {
					fmt.Sscanf(eq, "%f", &totalEquity)
				}
				if dp, ok := snapshot["daily_pnl"].(string); ok {
					fmt.Sscanf(dp, "%f", &dailyPnL)
				}
				if nd, ok := snapshot["net_delta"].(string); ok {
					fmt.Sscanf(nd, "%f", &netDelta)
				}
			}
		}

		// Calculate gauges
		dailyLossAbs := dailyPnL
		if dailyLossAbs > 0 {
			dailyLossAbs = 0
		} else {
			dailyLossAbs = -dailyLossAbs
		}

		dailyLossPct := float64(0)
		if maxDailyLoss > 0 {
			dailyLossPct = (dailyLossAbs / maxDailyLoss) * 100
		}

		dailyLossStatus := "ok"
		if dailyLossPct >= 75 {
			dailyLossStatus = "critical"
		} else if dailyLossPct >= 50 {
			dailyLossStatus = "warning"
		}

		// Find largest position % of equity
		var largestPositionPct float64
		posRows, posErr := pool.Query(ctx,
			"SELECT COALESCE(ABS(COALESCE(market_value, quantity * avg_cost)), 0) FROM positions WHERE account_id = 'default'")
		if posErr == nil {
			defer posRows.Close()
			for posRows.Next() {
				var mv float64
				if posRows.Scan(&mv) == nil && totalEquity > 0 {
					pct := (mv / totalEquity) * 100
					if pct > largestPositionPct {
						largestPositionPct = pct
					}
				}
			}
		}

		positionStatus := "ok"
		if largestPositionPct >= maxPositionPct*0.9 {
			positionStatus = "critical"
		} else if largestPositionPct >= maxPositionPct*0.6 {
			positionStatus = "warning"
		}

		deltaAbs := netDelta
		if deltaAbs < 0 {
			deltaAbs = -deltaAbs
		}
		deltaStatus := "ok"
		if maxDeltaExposure > 0 && deltaAbs >= maxDeltaExposure*0.9 {
			deltaStatus = "critical"
		} else if maxDeltaExposure > 0 && deltaAbs >= maxDeltaExposure*0.6 {
			deltaStatus = "warning"
		}

		gauges := []fiber.Map{
			{"name": "Daily Loss", "current": dailyLossAbs, "limit": maxDailyLoss, "unit": "$", "status": dailyLossStatus},
			{"name": "Largest Position", "current": largestPositionPct, "limit": maxPositionPct, "unit": "%", "status": positionStatus},
			{"name": "Net Delta", "current": deltaAbs, "limit": maxDeltaExposure, "unit": "", "status": deltaStatus},
			{"name": "Sector Concentration", "current": 0, "limit": maxSectorConcentration, "unit": "%", "status": "ok"},
		}

		// Determine circuit breaker state
		circuitBreakerState := "normal"
		if circuitBreakerEnabled && maxDailyLoss > 0 {
			switch {
			case dailyLossPct >= 150:
				circuitBreakerState = "emergency"
			case dailyLossPct >= 100:
				circuitBreakerState = "halt"
			case dailyLossPct >= 75:
				circuitBreakerState = "throttle"
			case dailyLossPct >= 50:
				circuitBreakerState = "warning"
			}
		}

		cb := fiber.Map{
			"state":    circuitBreakerState,
			"daily_pnl": fmt.Sprintf("%.2f", dailyPnL),
			"thresholds": fiber.Map{
				"warning":   maxDailyLoss * 0.5,
				"throttle":  maxDailyLoss * 0.75,
				"halt":      maxDailyLoss,
				"emergency": maxDailyLoss * 1.5,
			},
		}

		// Get recent risk checks
		checkRows, checkErr := pool.Query(ctx,
			"SELECT check_name, passed, reason, checked_at FROM risk_check_results ORDER BY id DESC LIMIT 20")
		var recentChecks []fiber.Map
		if checkErr == nil {
			defer checkRows.Close()
			for checkRows.Next() {
				var checkName, reason string
				var passed bool
				var checkedAt time.Time
				if checkRows.Scan(&checkName, &passed, &reason, &checkedAt) == nil {
					recentChecks = append(recentChecks, fiber.Map{
						"check_name": checkName,
						"passed":     passed,
						"message":    reason,
						"timestamp":  checkedAt.Format(time.RFC3339),
					})
				}
			}
		}
		if recentChecks == nil {
			recentChecks = []fiber.Map{}
		}

		return c.JSON(fiber.Map{
			"gauges":          gauges,
			"circuit_breaker": cb,
			"recent_checks":   recentChecks,
		})
	})

	api.Get("/risk/checks/:orderId", func(c *fiber.Ctx) error {
		orderID := c.Params("orderId")
		ctx, cancel := dbCtx()
		defer cancel()

		rows, err := pool.Query(ctx,
			"SELECT check_name, passed, reason, details FROM risk_check_results WHERE order_id = $1 ORDER BY id",
			orderID)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		defer rows.Close()

		var results []fiber.Map
		for rows.Next() {
			var checkName, reason string
			var passed bool
			var details *string
			if err := rows.Scan(&checkName, &passed, &reason, &details); err != nil {
				continue
			}
			result := fiber.Map{
				"check":  checkName,
				"passed": passed,
				"reason": reason,
			}
			if details != nil {
				var d map[string]interface{}
				if json.Unmarshal([]byte(*details), &d) == nil {
					result["details"] = d
				}
			}
			results = append(results, result)
		}

		if results == nil {
			results = []fiber.Map{}
		}

		return c.JSON(fiber.Map{"checks": results})
	})

	// ==================== KILL SWITCH ====================

	api.Post("/killswitch/activate", func(c *fiber.Ctx) error {
		var body struct {
			Reason string `json:"reason"`
		}
		c.BodyParser(&body)
		if body.Reason == "" {
			body.Reason = "Manual activation from dashboard"
		}

		ctx, cancel := dbCtx()
		defer cancel()

		rdb.Set(ctx, "killswitch:active", "true", 0)

		// Publish event
		data, _ := json.Marshal(map[string]interface{}{
			"type":      "killswitch",
			"active":    true,
			"reason":    body.Reason,
			"timestamp": time.Now().Format(time.RFC3339),
		})
		rdb.Publish(ctx, "killswitch:activated", string(data))
		rdb.Publish(ctx, "oms:killswitch", string(data))

		// Audit log
		auditDetails, _ := json.Marshal(map[string]interface{}{
			"reason": body.Reason,
		})
		pool.Exec(ctx,
			"INSERT INTO audit_log (action, entity_type, entity_id, details) VALUES ($1, $2, $3, $4)",
			"killswitch_activated", "system", "killswitch", auditDetails)

		return c.JSON(fiber.Map{"message": "Kill switch activated", "active": true})
	})

	api.Post("/killswitch/deactivate", func(c *fiber.Ctx) error {
		var body struct {
			Confirmation string `json:"confirmation"`
		}
		if err := c.BodyParser(&body); err != nil || body.Confirmation != "CONFIRM" {
			return c.Status(400).JSON(fiber.Map{"error": "Deactivation requires confirmation. Send {\"confirmation\": \"CONFIRM\"}"})
		}

		ctx, cancel := dbCtx()
		defer cancel()

		rdb.Del(ctx, "killswitch:active")

		data, _ := json.Marshal(map[string]interface{}{
			"type":      "killswitch",
			"active":    false,
			"timestamp": time.Now().Format(time.RFC3339),
		})
		rdb.Publish(ctx, "killswitch:deactivated", string(data))
		rdb.Publish(ctx, "oms:killswitch", string(data))

		auditDetails, _ := json.Marshal(map[string]interface{}{})
		pool.Exec(ctx,
			"INSERT INTO audit_log (action, entity_type, entity_id, details) VALUES ($1, $2, $3, $4)",
			"killswitch_deactivated", "system", "killswitch", auditDetails)

		return c.JSON(fiber.Map{"message": "Kill switch deactivated", "active": false})
	})

	api.Get("/killswitch/status", func(c *fiber.Ctx) error {
		ctx, cancel := dbCtx()
		defer cancel()

		val, err := rdb.Get(ctx, "killswitch:active").Result()
		active := err == nil && val == "true"

		return c.JSON(fiber.Map{"active": active})
	})

	// ==================== AUTONOMY ====================

	api.Get("/autonomy/level", func(c *fiber.Ctx) error {
		ctx, cancel := dbCtx()
		defer cancel()

		var value json.RawMessage
		err := pool.QueryRow(ctx,
			"SELECT value FROM app_config WHERE key = 'system.autonomy_level'").Scan(&value)
		if err != nil {
			return c.JSON(fiber.Map{"level": 1})
		}

		var level int
		json.Unmarshal(value, &level)
		descriptions := map[int]string{
			0: "Observer — signals are logged but no orders are created",
			1: "Advisor — each trade requires your approval",
			2: "Semi-Auto — trades approved symbols automatically, notifies after",
			3: "Autonomous — trades freely within risk limits",
			4: "Adaptive — autonomous with parameter optimization",
		}

		return c.JSON(fiber.Map{
			"level":       level,
			"description": descriptions[level],
		})
	})

	api.Put("/autonomy/level", func(c *fiber.Ctx) error {
		var body struct {
			Level int `json:"level"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
		}
		if body.Level < 0 || body.Level > 4 {
			return c.Status(400).JSON(fiber.Map{"error": "level must be between 0 and 4"})
		}

		ctx, cancel := dbCtx()
		defer cancel()

		levelJSON, _ := json.Marshal(body.Level)
		pool.Exec(ctx,
			"UPDATE app_config SET value=$1, updated_at=NOW() WHERE key='system.autonomy_level'",
			levelJSON)

		rdb.Publish(ctx, "config:changed", `{"key":"system.autonomy_level"}`)

		return c.JSON(fiber.Map{"message": "autonomy level updated", "level": body.Level})
	})

	api.Get("/autonomy/pending", func(c *fiber.Ctx) error {
		ctx, cancel := dbCtx()
		defer cancel()

		rows, err := pool.Query(ctx,
			`SELECT order_id, symbol, side, order_type, quantity, limit_price, strategy_id, created_at
			 FROM orders WHERE status = 'AWAITING_APPROVAL' ORDER BY created_at DESC`)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		defer rows.Close()

		var pending []fiber.Map
		for rows.Next() {
			var orderID, symbol, side, orderType, strategyID string
			var quantity int
			var limitPrice *float64
			var createdAt time.Time

			if err := rows.Scan(&orderID, &symbol, &side, &orderType, &quantity, &limitPrice, &strategyID, &createdAt); err != nil {
				continue
			}

			elapsed := time.Since(createdAt)
			expiresIn := 300 - int(elapsed.Seconds())
			if expiresIn < 0 {
				expiresIn = 0
			}

			order := fiber.Map{
				"order_id":           orderID,
				"symbol":             symbol,
				"side":               side,
				"order_type":         orderType,
				"quantity":           quantity,
				"strategy_id":        strategyID,
				"created_at":         createdAt.Format(time.RFC3339),
				"expires_in_seconds": expiresIn,
			}
			if limitPrice != nil {
				order["limit_price"] = *limitPrice
			}

			pending = append(pending, order)
		}

		if pending == nil {
			pending = []fiber.Map{}
		}

		return c.JSON(fiber.Map{"pending": pending, "count": len(pending)})
	})
}

func proxyToOMS(c *fiber.Ctx, method, path string) error {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(omsBaseURL + path)
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "OMS service unreachable",
		})
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	if json.Unmarshal(body, &result) == nil {
		return c.JSON(result)
	}
	return c.Send(body)
}
