package alpaca

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gorilla/websocket"
)

type authMessage struct {
	Action string `json:"action"`
	Key    string `json:"key"`
	Secret string `json:"secret"`
}

type wsMessage struct {
	T   string `json:"T"`
	Msg string `json:"msg,omitempty"`
	// Subscription confirmation fields
	Trades []string `json:"trades,omitempty"`
	Quotes []string `json:"quotes,omitempty"`
	Bars   []string `json:"bars,omitempty"`
}

// authenticate performs the Alpaca WebSocket auth handshake.
// Expects: receive "connected" → send auth → receive "authenticated".
func authenticate(conn *websocket.Conn, apiKey, apiSecret string) error {
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))

	// Read initial "connected" message
	_, msg, err := conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("failed to read welcome message: %w", err)
	}

	var msgs []wsMessage
	if err := json.Unmarshal(msg, &msgs); err != nil {
		return fmt.Errorf("failed to parse welcome message: %w", err)
	}

	if len(msgs) == 0 || msgs[0].T != "success" || msgs[0].Msg != "connected" {
		return fmt.Errorf("unexpected welcome message: %s", string(msg))
	}

	// Send auth
	auth := authMessage{
		Action: "auth",
		Key:    apiKey,
		Secret: apiSecret,
	}
	if err := conn.WriteJSON(auth); err != nil {
		return fmt.Errorf("failed to send auth: %w", err)
	}

	// Read auth response
	_, msg, err = conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("failed to read auth response: %w", err)
	}

	if err := json.Unmarshal(msg, &msgs); err != nil {
		return fmt.Errorf("failed to parse auth response: %w", err)
	}

	if len(msgs) == 0 {
		return fmt.Errorf("empty auth response")
	}

	if msgs[0].T == "error" {
		return fmt.Errorf("auth failed: %s", msgs[0].Msg)
	}

	if msgs[0].T != "success" || msgs[0].Msg != "authenticated" {
		return fmt.Errorf("unexpected auth response: %s", string(msg))
	}

	conn.SetReadDeadline(time.Time{}) // clear deadline
	return nil
}
