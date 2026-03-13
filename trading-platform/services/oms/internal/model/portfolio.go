package model

import (
	"time"

	"github.com/shopspring/decimal"
)

type PortfolioSnapshot struct {
	Timestamp        time.Time       `json:"timestamp"`
	AccountID        string          `json:"account_id"`
	TotalEquity      decimal.Decimal `json:"total_equity"`
	Cash             decimal.Decimal `json:"cash"`
	BuyingPower      decimal.Decimal `json:"buying_power"`
	UnrealizedPnL    decimal.Decimal `json:"unrealized_pnl"`
	RealizedPnLToday decimal.Decimal `json:"realized_pnl_today"`
	DailyPnL         decimal.Decimal `json:"daily_pnl"`
	Positions        []Position      `json:"positions"`
	NetDelta         decimal.Decimal `json:"net_delta"`
	NetGamma         decimal.Decimal `json:"net_gamma"`
	NetTheta         decimal.Decimal `json:"net_theta"`
	NetVega          decimal.Decimal `json:"net_vega"`
	PositionCount    int             `json:"position_count"`
	LongExposure     decimal.Decimal `json:"long_exposure"`
	ShortExposure    decimal.Decimal `json:"short_exposure"`
	NetExposure      decimal.Decimal `json:"net_exposure"`
	GrossExposure    decimal.Decimal `json:"gross_exposure"`
}
