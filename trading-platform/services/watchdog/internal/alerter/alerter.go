package alerter

import (
	"os"

	"go.uber.org/zap"
)

// Alerter handles critical service alerts.
type Alerter struct {
	logger     *zap.Logger
	alertEmail string
}

// New creates a new Alerter instance.
func New(logger *zap.Logger) *Alerter {
	return &Alerter{
		logger:     logger,
		alertEmail: os.Getenv("ALERT_EMAIL"),
	}
}

// AlertCritical logs a critical alert for a service. If ALERT_EMAIL is
// configured, it logs that an email notification would be sent.
func (a *Alerter) AlertCritical(serviceName, details string) {
	a.logger.Error("CRITICAL ALERT",
		zap.String("service", serviceName),
		zap.String("details", details),
	)

	if a.alertEmail != "" {
		a.logger.Info("would send alert email",
			zap.String("to", a.alertEmail),
			zap.String("service", serviceName),
			zap.String("details", details),
		)
	}
}
