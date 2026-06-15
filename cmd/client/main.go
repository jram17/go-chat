package main

import (
	"bufio"
	"crypto/tls"
	"flag"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	client "github.com/jram17/go-chat/internal/client"
	"github.com/jram17/go-chat/internal/crypto"
	"github.com/jram17/go-chat/internal/protocol"
)

func main() {
	addr := flag.String("addr", "localhost:9000", "server address (host:port)")
	name := flag.String("name", "", "username (prompted if not provided)")
	flag.Parse()

	config := &tls.Config{
		InsecureSkipVerify: true,
	}
	conn, err := tls.Dial("tcp", *addr, config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to %s: %v\n", *addr, err)
		os.Exit(1)
	}
	defer conn.Close()

	// Get username
	username := *name
	if username == "" {
		fmt.Print("Enter your name: ")
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		username = scanner.Text()
	}

	if username == "" {
		fmt.Fprintln(os.Stderr, "Username cannot be empty")
		os.Exit(1)
	}

	// Send join
	join := protocol.Envelope{
		Type:      protocol.MessageTypeJoin,
		From:      username,
		Timestamp: time.Now().Unix(),
	}
	encoded, err := protocol.Encode(join)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Encoding error: %v\n", err)
		os.Exit(1)
	}
	conn.Write(encoded)

	// Generate keys and send key exchange
	privateKey, pub, err := crypto.GenerateKeyPair()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Key generation failed: %v\n", err)
		os.Exit(1)
	}
	keyEnv := protocol.Envelope{
		Type:      protocol.MessageTypeKeyExchange,
		From:      username,
		Payload:   pub,
		Timestamp: time.Now().Unix(),
	}
	encoded, err = protocol.Encode(keyEnv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Encoding error: %v\n", err)
		os.Exit(1)
	}
	conn.Write(encoded)

	// Create TUI model
	peerKeys := make(map[string][]byte)
	m := client.NewModel(conn, username, privateKey, peerKeys)

	// Start TUI
	p := tea.NewProgram(m, tea.WithAltScreen())

	// Reader goroutine: reads from server and sends to TUI
	go func() {
		reader := bufio.NewReader(conn)
		for {
			env, err := protocol.Decode(reader)
			if err != nil {
				p.Send(client.ErrMsg{Err: err})
				return
			}
			p.Send(client.IncomingMsg{Env: env})
		}
	}()

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}
}
