package routes

import (
	"context"
	"time"

	"trading-platform/gateway/internal/auth"
	"trading-platform/gateway/internal/health"
	"trading-platform/gateway/internal/ws"
	"trading-platform/gateway/pkg/redis"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

const dbTimeout = 5 * time.Second

func dbCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), dbTimeout)
}

func Setup(app *fiber.App, pool *pgxpool.Pool, rdb *redis.Client, jwtMgr *auth.JWTManager, hub *ws.Hub, log *zap.SugaredLogger, startTime time.Time) {
	healthHandler := health.NewHandler(pool, rdb.Client, startTime)
	configHandler := NewConfigHandler(pool, rdb, log)

	// Public health endpoints
	app.Get("/healthz", healthHandler.Healthz)
	app.Get("/readyz", healthHandler.Readyz)

	// Auth endpoints
	app.Post("/api/auth/login", loginHandler(pool, jwtMgr, log))
	app.Post("/api/auth/setup", setupHandler(pool, jwtMgr, log))

	// Public health routes (no auth required for dashboard)
	app.Get("/api/health/services", healthServicesHandler(pool))
	app.Get("/api/health/resources", healthResourcesHandler())

	// Protected API routes
	api := app.Group("/api", auth.AuthMiddleware(jwtMgr))

	// Config routes
	api.Get("/config", configHandler.GetAll)
	api.Get("/config/:key", configHandler.GetByKey)
	api.Put("/config/:key", configHandler.Update)
	api.Post("/config/bulk", configHandler.BulkUpdate)
}

func loginHandler(pool *pgxpool.Pool, jwtMgr *auth.JWTManager, log *zap.SugaredLogger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var body struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
		}

		ctx, cancel := dbCtx()
		defer cancel()

		var userID, passwordHash, role string
		err := pool.QueryRow(ctx,
			"SELECT user_id, password_hash, role FROM users WHERE username = $1", body.Username).
			Scan(&userID, &passwordHash, &role)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid credentials"})
		}

		if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(body.Password)); err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid credentials"})
		}

		// Update last login
		ctx2, cancel2 := dbCtx()
		defer cancel2()
		_, _ = pool.Exec(ctx2,
			"UPDATE users SET last_login = NOW() WHERE user_id = $1", userID)

		token, err := jwtMgr.GenerateToken(userID, body.Username, role)
		if err != nil {
			log.Errorw("Failed to generate token", "error", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to generate token"})
		}

		return c.JSON(fiber.Map{
			"token":    token,
			"username": body.Username,
			"role":     role,
		})
	}
}

func setupHandler(pool *pgxpool.Pool, jwtMgr *auth.JWTManager, log *zap.SugaredLogger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx, cancel := dbCtx()
		defer cancel()

		// Only allow if no users exist
		var count int
		err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
		if err != nil {
			log.Errorw("Failed to count users", "error", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "database error"})
		}

		if count > 0 {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "setup already completed"})
		}

		var body struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
		}

		if len(body.Username) < 3 || len(body.Password) < 8 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "username must be at least 3 characters, password at least 8",
			})
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to hash password"})
		}

		ctx2, cancel2 := dbCtx()
		defer cancel2()

		var userID string
		err = pool.QueryRow(ctx2,
			"INSERT INTO users (username, password_hash, role) VALUES ($1, $2, 'admin') RETURNING user_id",
			body.Username, string(hash)).Scan(&userID)
		if err != nil {
			log.Errorw("Failed to create admin user", "error", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create user"})
		}

		token, err := jwtMgr.GenerateToken(userID, body.Username, "admin")
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to generate token"})
		}

		log.Infow("Admin user created via setup", "username", body.Username)

		return c.Status(fiber.StatusCreated).JSON(fiber.Map{
			"token":    token,
			"username": body.Username,
			"role":     "admin",
			"message":  "Admin user created successfully",
		})
	}
}

func healthServicesHandler(pool *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx, cancel := dbCtx()
		defer cancel()

		rows, err := pool.Query(ctx,
			"SELECT service_name, status, last_check, response_time_ms, details FROM service_health")
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		defer rows.Close()

		var services []fiber.Map
		for rows.Next() {
			var name, status string
			var lastCheck time.Time
			var responseTimeMs *int
			var details *string
			if err := rows.Scan(&name, &status, &lastCheck, &responseTimeMs, &details); err != nil {
				continue
			}
			svc := fiber.Map{
				"name":       name,
				"status":     status,
				"last_check": lastCheck.Format(time.RFC3339),
			}
			if responseTimeMs != nil {
				svc["response_time_ms"] = *responseTimeMs
			}
			services = append(services, svc)
		}

		if services == nil {
			services = []fiber.Map{}
		}

		return c.JSON(fiber.Map{"services": services})
	}
}

func healthResourcesHandler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"resources": []fiber.Map{},
			"note":      "Resource monitoring coming in a future update",
		})
	}
}
