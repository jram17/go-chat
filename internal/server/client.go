package server

import (
	"bufio"
	"log/slog"
	"net"
	"time"

	"github.com/jram17/go-chat/internal/protocol"
)

const maxMessageSize = 65536 // 64KB

type Client struct {
	conn      net.Conn
	send      chan protocol.Envelope
	hub       *Hub
	username  string
	publicKey []byte
	logger    *slog.Logger
}

func NewClient(conn net.Conn, hub *Hub, logger *slog.Logger) *Client {
	return &Client{
		conn:   conn,
		send:   make(chan protocol.Envelope, 256),
		hub:    hub,
		logger: logger,
	}
}

func (c *Client) ReadPump() {
	defer func() {
		if c.username != "" {
			c.hub.Broadcast(protocol.Envelope{
				Type:      protocol.MessageTypeLeave,
				From:      c.username,
				Timestamp: time.Now().Unix(),
			})
		}
		c.hub.Unregister(c)
		c.conn.Close()
	}()

	reader := bufio.NewReader(c.conn)
	for {
		env, err := protocol.Decode(reader)
		if err != nil {
			c.logger.Info("client disconnected", "addr", c.conn.RemoteAddr(), "username", c.username)
			return
		}

		// Validate message size
		if len(env.Payload) > maxMessageSize {
			c.logger.Warn("message too large, dropping", "username", c.username, "size", len(env.Payload))
			continue
		}

		switch env.Type {
		case protocol.MessageTypeJoin:
			if c.username != "" {
				continue
			}
			c.username = env.From
			c.hub.clients[c.username] = c
			c.logger.Info("client joined", "username", c.username, "addr", c.conn.RemoteAddr())
			c.hub.Broadcast(protocol.Envelope{
				Type:      protocol.MessageTypeJoin,
				From:      c.username,
				Timestamp: time.Now().Unix(),
			})

		case protocol.MessageTypeChat:
			env.From = c.username
			env.Timestamp = time.Now().Unix()
			c.hub.Broadcast(env)

		case protocol.MessageTypeKeyExchange:
			c.publicKey = env.Payload
			c.hub.HandleKeyExchange(c)

		case protocol.MessageTypePrivate:
			env.From = c.username
			env.Timestamp = time.Now().Unix()
			c.hub.SendPrivates(env)

		case protocol.MessageTypeTyping:
			env.From = c.username
			c.hub.SendPrivates(env)

		case protocol.MessageTypeUserList:
			c.hub.RequestUserList(c)

		default:
			c.logger.Warn("unknown message type", "type", env.Type, "username", c.username)
		}
	}
}

func (c *Client) WritePump() {
	defer c.conn.Close()
	for message := range c.send {
		encoded, err := protocol.Encode(message)
		if err != nil {
			c.logger.Error("encoding error", "err", err, "username", c.username)
			continue
		}
		_, err = c.conn.Write(encoded)
		if err != nil {
			c.logger.Info("write error, disconnecting", "addr", c.conn.RemoteAddr(), "username", c.username)
			return
		}
	}
}
