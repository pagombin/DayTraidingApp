package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"trading-platform/watchdog/internal/alerter"
	"trading-platform/watchdog/internal/docker"
)

// Service represents a registered service to monitor.
type Service struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"` // "http" or "tcp"
	URL      string `json:"url"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Enabled  bool   `json:"enabled"`
}

// HealthUpdate is published to Redis on each health check.
type HealthUpdate struct {
	ServiceName string    `json:"service_name"`
	Status      string    `json:"status"` // "healthy" or "unhealthy"
	Message     string    `json:"message,omitempty"`
	CheckedAt   time.Time `json:"checked_at"`
}

// Monitor checks registered services and handles self-healing.
type Monitor struct {
	logger       *zap.Logger
	db           *pgxpool.Pool
	rdb          *redis.Client
	alerter      *alerter.Alerter
	httpClient   *http.Client
	pollInterval time.Duration

	mu                  sync.Mutex
	consecutiveFailures map[string]int
	restartAttempts     map[string]int
}

// New creates a new Monitor instance.
func New(logger *zap.Logger, db *pgxpool.Pool, rdb *redis.Client, alt *alerter.Alerter) *Monitor {
	interval := 10 * time.Second
	if val, ok := os.LookupEnv("POLL_INTERVAL"); ok {
		if secs, err := strconv.Atoi(val); err == nil && secs > 0 {
			interval = time.Duration(secs) * time.Second
		}
	}

	return &Monitor{
		logger: logger,
		db:     db,
		rdb:    rdb,
		alerter: alt,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		pollInterval:        interval,
		consecutiveFailures: make(map[string]int),
		restartAttempts:     make(map[string]int),
	}
}

// Start begins the monitoring loop. It blocks until ctx is cancelled.
func (m *Monitor) Start(ctx context.Context) {
	m.logger.Info("monitor started", zap.Duration("poll_interval", m.pollInterval))

	ticker := time.NewTicker(m.pollInterval)
	defer ticker.Stop()

	// Run immediately on start, then on each tick.
	m.runChecks(ctx)

	for {
		select {
		case <-ctx.Done():
			m.logger.Info("monitor stopped")
			return
		case <-ticker.C:
			m.runChecks(ctx)
		}
	}
}

func (m *Monitor) runChecks(ctx context.Context) {
	services, err := m.loadServices(ctx)
	if err != nil {
		m.logger.Error("failed to load registered services", zap.Error(err))
		return
	}

	for _, svc := range services {
		if !svc.Enabled {
			continue
		}
		m.checkService(ctx, svc)
	}
}

func (m *Monitor) loadServices(ctx context.Context) ([]Service, error) {
	rows, err := m.db.Query(ctx,
		`SELECT id, name, type, COALESCE(url, ''), COALESCE(host, ''), COALESCE(port, 0), enabled
		 FROM registered_services WHERE enabled = true`)
	if err != nil {
		return nil, fmt.Errorf("query registered_services: %w", err)
	}
	defer rows.Close()

	var services []Service
	for rows.Next() {
		var s Service
		if err := rows.Scan(&s.ID, &s.Name, &s.Type, &s.URL, &s.Host, &s.Port, &s.Enabled); err != nil {
			return nil, fmt.Errorf("scan service row: %w", err)
		}
		services = append(services, s)
	}
	return services, rows.Err()
}

func (m *Monitor) checkService(ctx context.Context, svc Service) {
	var status string
	var message string

	switch svc.Type {
	case "http":
		status, message = m.checkHTTP(ctx, svc)
	case "tcp":
		status, message = m.checkTCP(svc)
	default:
		m.logger.Warn("unknown service type", zap.String("service", svc.Name), zap.String("type", svc.Type))
		return
	}

	// Record health result in database (UPSERT).
	m.recordHealth(ctx, svc, status, message)

	// Publish health update to Redis.
	update := HealthUpdate{
		ServiceName: svc.Name,
		Status:      status,
		Message:     message,
		CheckedAt:   time.Now().UTC(),
	}
	m.publishHealthUpdate(ctx, update)

	// Handle failure tracking and self-healing.
	m.mu.Lock()
	defer m.mu.Unlock()

	if status == "unhealthy" {
		m.consecutiveFailures[svc.Name]++
		failures := m.consecutiveFailures[svc.Name]

		m.logger.Warn("service health check failed",
			zap.String("service", svc.Name),
			zap.Int("consecutive_failures", failures),
			zap.String("message", message),
		)

		if failures >= 3 {
			restarts := m.restartAttempts[svc.Name]

			if restarts >= 3 {
				m.logger.Error("CRITICAL: service unrecoverable after restart attempts",
					zap.String("service", svc.Name),
					zap.Int("restart_attempts", restarts),
				)
				m.alerter.AlertCritical(svc.Name,
					fmt.Sprintf("service %s has failed %d consecutive checks and %d restart attempts",
						svc.Name, failures, restarts))
				return
			}

			m.logger.Warn("attempting Docker restart for service",
				zap.String("service", svc.Name),
				zap.Int("attempt", restarts+1),
			)

			if err := docker.RestartContainer(ctx, svc.Name); err != nil {
				m.logger.Error("failed to restart container",
					zap.String("service", svc.Name),
					zap.Error(err),
				)
				m.restartAttempts[svc.Name]++
			} else {
				m.logger.Info("container restarted successfully",
					zap.String("service", svc.Name),
				)
				// Reset failure count after successful restart; keep restart attempts count.
				m.consecutiveFailures[svc.Name] = 0
				m.restartAttempts[svc.Name]++
			}
		}
	} else {
		// Service is healthy; reset counters.
		m.consecutiveFailures[svc.Name] = 0
		m.restartAttempts[svc.Name] = 0
	}
}

func (m *Monitor) checkHTTP(ctx context.Context, svc Service) (string, string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, svc.URL, nil)
	if err != nil {
		return "unhealthy", fmt.Sprintf("failed to create request: %v", err)
	}

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return "unhealthy", fmt.Sprintf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "unhealthy", fmt.Sprintf("unexpected status code: %d", resp.StatusCode)
	}
	return "healthy", ""
}

func (m *Monitor) checkTCP(svc Service) (string, string) {
	addr := fmt.Sprintf("%s:%d", svc.Host, svc.Port)
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return "unhealthy", fmt.Sprintf("tcp dial failed: %v", err)
	}
	conn.Close()
	return "healthy", ""
}

func (m *Monitor) recordHealth(ctx context.Context, svc Service, status, message string) {
	_, err := m.db.Exec(ctx,
		`INSERT INTO service_health (service_id, service_name, status, message, checked_at)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (service_name)
		 DO UPDATE SET status = EXCLUDED.status,
		               message = EXCLUDED.message,
		               checked_at = EXCLUDED.checked_at`,
		svc.ID, svc.Name, status, message, time.Now().UTC())
	if err != nil {
		m.logger.Error("failed to record health check",
			zap.String("service", svc.Name),
			zap.Error(err),
		)
	}
}

func (m *Monitor) publishHealthUpdate(ctx context.Context, update HealthUpdate) {
	data, err := json.Marshal(update)
	if err != nil {
		m.logger.Error("failed to marshal health update", zap.Error(err))
		return
	}

	if err := m.rdb.Publish(ctx, "health:updates", string(data)).Err(); err != nil {
		m.logger.Error("failed to publish health update to Redis", zap.Error(err))
	}
}
