package ws

import (
	"net/http"

	"github.com/fasthttp/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/valyala/fasthttp/fasthttpadaptor"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins in development
	},
}

func SetupRoutes(app *fiber.App, hub *Hub) {
	app.Get("/ws", func(c *fiber.Ctx) error {
		fasthttpadaptor.NewFastHTTPHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				return
			}

			client := NewClient(hub, conn)
			hub.Register(client)

			go client.WritePump()
			go client.ReadPump()
		}))(c.Context())
		return nil
	})
}
