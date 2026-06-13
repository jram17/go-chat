package server

import "github.com/jram17/go-chat/internal/protocol"

type Hub struct {
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
	broadcasts chan protocol.Envelope
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcasts: make(chan protocol.Envelope),
	}
}
func (h *Hub) Register(client *Client) {
	h.register <- client
}
func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}
func (h *Hub) Broadcast(env protocol.Envelope) {
	h.broadcasts <- env
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
		case message := <-h.broadcasts:
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					//slow/ dead clients - drop them
					delete(h.clients, client)
					close(client.send)
				}
			}
		}
	}

}
