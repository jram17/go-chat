package main

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	client "github.com/jram17/go-chat/internal/client"
	"github.com/jram17/go-chat/internal/crypto"
	"github.com/jram17/go-chat/internal/protocol"
)

func main() {
	config := &tls.Config{
		InsecureSkipVerify: true,
	}
	conn, err := tls.Dial("tcp", "localhost:9000", config)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	// Get username before starting TUI
	fmt.Print("Enter your name: ")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	username := scanner.Text()

	// Send join
	join := protocol.Envelope{
		Type:      protocol.MessageTypeJoin,
		From:      username,
		Timestamp: time.Now().Unix(),
	}
	encoded, err := protocol.Encode(join)
	if err != nil {
		panic(err)
	}
	conn.Write(encoded)

	// Generate keys and send key exchange
	privateKey, pub, err := crypto.GenerateKeyPair()
	if err != nil {
		panic(err)
	}
	keyEnv := protocol.Envelope{
		Type:      protocol.MessageTypeKeyExchange,
		From:      username,
		Payload:   pub,
		Timestamp: time.Now().Unix(),
	}
	encoded, err = protocol.Encode(keyEnv)
	if err != nil {
		panic(err)
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
		fmt.Println("Error running TUI:", err)
		os.Exit(1)
	}
}
