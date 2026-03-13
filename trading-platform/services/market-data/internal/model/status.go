package model

import "time"

type ConnectionState string

const (
	StateConnecting   ConnectionState = "connecting"
	StateConnected    ConnectionState = "connected"
	StateDisconnected ConnectionState = "disconnected"
	StateReconnecting ConnectionState = "reconnecting"
	StateError        ConnectionState = "error"
)

type SymbolStatus struct {
	Symbol       string    `json:"symbol"`
	LastTickTime time.Time `json:"last_tick_time"`
	TickCount    int64     `json:"tick_count"`
	IsStale      bool      `json:"is_stale"`
}

type ServiceStatus struct {
	State           ConnectionState `json:"state"`
	Provider        string          `json:"provider"`
	ConnectedSince  *time.Time      `json:"connected_since,omitempty"`
	SymbolCount     int             `json:"symbol_count"`
	TicksPerSecond  float64         `json:"ticks_per_second"`
	TotalTicksToday int64           `json:"total_ticks_today"`
	StaleSymbols    []string        `json:"stale_symbols,omitempty"`
	LastError       string          `json:"last_error,omitempty"`
	LastErrorTime   *time.Time      `json:"last_error_time,omitempty"`
	Uptime          string          `json:"uptime"`
}
