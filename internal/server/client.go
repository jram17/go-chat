package server

import (
	"bufio"
	"fmt"
	"net"
	"time"

	"github.com/jram17/go-chat/internal/protocol"
)

type Client struct {
	conn      net.Conn
	send      chan protocol.Envelope
	hub       *Hub
	username  string
	publicKey []byte
}

func NewClient(conn net.Conn, hub *Hub) *Client {
	return &Client{
		conn:     conn,
		send:     make(chan protocol.Envelope, 256),
		hub:      hub,
		username: "",
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
			fmt.Println("client disconnected:", c.conn.RemoteAddr())
			return
		}
		switch env.Type {
		case protocol.MessageTypeJoin:
			if c.username != "" {
				continue
			}
			c.username = env.From
			c.hub.clients[c.username] = c
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

		default:
			fmt.Println("unknown message type:", env.Type)
		}
	}
}

func (c *Client) WritePump() {
	defer c.conn.Close()
	for message := range c.send {
		encoded, err := protocol.Encode(message)
		if err != nil {
			fmt.Println("encoding error:", err)
		}
		_, err = c.conn.Write(encoded)
		if err != nil {
			fmt.Println("write err:", c.conn.RemoteAddr())
			return
		}
	}
}
