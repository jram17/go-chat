package server

import (
	"log/slog"
	"strings"
	"time"

	"github.com/jram17/go-chat/internal/protocol"
)

type Hub struct {
	clients      map[string]*Client
	register     chan *Client
	unregister   chan *Client
	broadcasts   chan protocol.Envelope
	privates     chan protocol.Envelope
	keyExchanges chan *Client
	userList     chan *Client
	logger       *slog.Logger
}

func NewHub(logger *slog.Logger) *Hub {
	return &Hub{
		clients:      make(map[string]*Client),
		register:     make(chan *Client),
		unregister:   make(chan *Client),
		broadcasts:   make(chan protocol.Envelope),
		privates:     make(chan protocol.Envelope),
		keyExchanges: make(chan *Client),
		userList:     make(chan *Client),
		logger:       logger,
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
func (h *Hub) SendPrivates(env protocol.Envelope) {
	h.privates <- env
}
func (h *Hub) HandleKeyExchange(client *Client) {
	h.keyExchanges <- client
}
func (h *Hub) RequestUserList(client *Client) {
	h.userList <- client
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.userList:
			usernames := make([]string, 0, len(h.clients))
			for name := range h.clients {
				usernames = append(usernames, name)
			}
			client.send <- protocol.Envelope{
				Type:      protocol.MessageTypeUserList,
				From:      "server",
				Payload:   []byte(strings.Join(usernames, ",")),
				Timestamp: time.Now().Unix(),
			}

		case client := <-h.keyExchanges:
			for _, existing := range h.clients {
				if existing.username == client.username {
					continue
				}
				if existing.publicKey == nil {
					continue
				}
				client.send <- protocol.Envelope{
					Type:    protocol.MessageTypeKeyExchange,
					From:    existing.username,
					Payload: existing.publicKey,
				}
			}
			for _, existing := range h.clients {
				if existing.username == client.username {
					continue
				}
				existing.send <- protocol.Envelope{
					Type:    protocol.MessageTypeKeyExchange,
					From:    client.username,
					Payload: client.publicKey,
				}
			}

		case env := <-h.privates:
			recipient, ok := h.clients[env.To]
			if ok {
				recipient.send <- env
			} else {
				h.logger.Warn("private message recipient not found", "to", env.To, "from", env.From)
			}

		case client := <-h.unregister:
			if _, ok := h.clients[client.username]; ok {
				h.logger.Info("client unregistered", "username", client.username)
				delete(h.clients, client.username)
				close(client.send)
			}

		case message := <-h.broadcasts:
			for _, client := range h.clients {
				select {
				case client.send <- message:
				default:
					h.logger.Warn("dropping slow client", "username", client.username)
					delete(h.clients, client.username)
					close(client.send)
				}
			}
		}
	}
}
