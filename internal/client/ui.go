package client

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jram17/go-chat/internal/crypto"
	"github.com/jram17/go-chat/internal/protocol"
)

var (
	senderStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
	privateStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	systemStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
)

// IncomingMsg is sent from the network reader goroutine to the TUI
type IncomingMsg struct {
	Env protocol.Envelope
}

// ErrMsg is sent when the connection is lost
type ErrMsg struct {
	Err error
}

type Model struct {
	viewport   viewport.Model
	textarea   textarea.Model
	messages   []string
	conn       net.Conn
	username   string
	privateKey []byte
	peerKeys   map[string][]byte
	ready      bool
	width      int
	height     int
}

func NewModel(conn net.Conn, username string, privateKey []byte, peerKeys map[string][]byte) Model {
	ta := textarea.New()
	ta.Placeholder = "Type a message..."
	ta.Focus()
	ta.SetHeight(1)
	ta.ShowLineNumbers = false
	ta.KeyMap.InsertNewline.SetEnabled(false)

	vp := viewport.New(80, 20)

	return Model{
		viewport:   vp,
		textarea:   ta,
		messages:   []string{},
		conn:       conn,
		username:   username,
		privateKey: privateKey,
		peerKeys:   peerKeys,
	}
}

func (m Model) Init() tea.Cmd {
	return textarea.Blink
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		vpCmd tea.Cmd
		taCmd tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - 4 // leave room for textarea + divider
		m.textarea.SetWidth(msg.Width)
		m.ready = true
		m.viewport.SetContent(strings.Join(m.messages, "\n"))
		m.viewport.GotoBottom()

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			input := m.textarea.Value()
			if input == "" {
				return m, nil
			}
			m.textarea.Reset()
			m.sendMessage(input)
			return m, nil
		}

	case IncomingMsg:
		m.handleIncoming(msg.Env)
		m.viewport.SetContent(strings.Join(m.messages, "\n"))
		m.viewport.GotoBottom()

	case ErrMsg:
		m.appendMsg(errorStyle.Render("Disconnected from server"))
		m.viewport.SetContent(strings.Join(m.messages, "\n"))
		m.viewport.GotoBottom()
	}

	m.viewport, vpCmd = m.viewport.Update(msg)
	m.textarea, taCmd = m.textarea.Update(msg)

	return m, tea.Batch(vpCmd, taCmd)
}

func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}
	divider := lipgloss.NewStyle().
		Width(m.width).
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		Render("")

	return fmt.Sprintf("%s\n%s\n%s",
		m.viewport.View(),
		divider,
		m.textarea.View(),
	)
}

func (m *Model) appendMsg(msg string) {
	m.messages = append(m.messages, msg)
}

func (m *Model) handleIncoming(env protocol.Envelope) {
	switch env.Type {
	case protocol.MessageTypeJoin:
		m.appendMsg(systemStyle.Render(fmt.Sprintf("● %s joined the chat", env.From)))

	case protocol.MessageTypeLeave:
		m.appendMsg(systemStyle.Render(fmt.Sprintf("● %s left the chat", env.From)))

	case protocol.MessageTypeChat:
		m.appendMsg(fmt.Sprintf("%s %s",
			senderStyle.Render(fmt.Sprintf("[%s]:", env.From)),
			string(env.Payload)))

	case protocol.MessageTypePrivate:
		senderPub, ok := m.peerKeys[env.From]
		if !ok {
			m.appendMsg(errorStyle.Render("sender public key not found"))
			return
		}
		key, err := crypto.ComputeSharedSecret(m.privateKey, senderPub)
		if err != nil {
			m.appendMsg(errorStyle.Render("key derivation failed"))
			return
		}
		plaintext, err := crypto.Decrypt(key, env.Payload)
		if err != nil {
			m.appendMsg(errorStyle.Render("decryption failed"))
			return
		}
		m.appendMsg(fmt.Sprintf("%s %s",
			privateStyle.Render(fmt.Sprintf("[PRIVATE][%s]:", env.From)),
			string(plaintext)))

	case protocol.MessageTypeKeyExchange:
		m.peerKeys[env.From] = env.Payload
		m.appendMsg(systemStyle.Render(fmt.Sprintf("● Key received from %s", env.From)))
	}
}

func (m *Model) sendMessage(input string) {
	var env protocol.Envelope

	if strings.HasPrefix(input, "/msg") {
		parts := strings.SplitN(input, " ", 3)
		if len(parts) < 3 {
			m.appendMsg(errorStyle.Render("Usage: /msg <user> <message>"))
			m.viewport.SetContent(strings.Join(m.messages, "\n"))
			m.viewport.GotoBottom()
			return
		}
		recipient := parts[1]
		message := parts[2]

		recipientPub, ok := m.peerKeys[recipient]
		if !ok {
			m.appendMsg(errorStyle.Render(fmt.Sprintf("public key not found for %s", recipient)))
			m.viewport.SetContent(strings.Join(m.messages, "\n"))
			m.viewport.GotoBottom()
			return
		}
		key, err := crypto.ComputeSharedSecret(m.privateKey, recipientPub)
		if err != nil {
			m.appendMsg(errorStyle.Render("key derivation failed"))
			m.viewport.SetContent(strings.Join(m.messages, "\n"))
			m.viewport.GotoBottom()
			return
		}
		ciphertext, err := crypto.Encrypt(key, []byte(message))
		if err != nil {
			m.appendMsg(errorStyle.Render("encryption failed"))
			m.viewport.SetContent(strings.Join(m.messages, "\n"))
			m.viewport.GotoBottom()
			return
		}

		env = protocol.Envelope{
			Type:      protocol.MessageTypePrivate,
			From:      m.username,
			To:        recipient,
			Payload:   ciphertext,
			Timestamp: time.Now().Unix(),
		}
		m.appendMsg(fmt.Sprintf("%s %s",
			privateStyle.Render(fmt.Sprintf("[→ %s]:", recipient)),
			message))
	} else {
		env = protocol.Envelope{
			Type:      protocol.MessageTypeChat,
			From:      m.username,
			Payload:   []byte(input),
			Timestamp: time.Now().Unix(),
		}
		m.appendMsg(fmt.Sprintf("%s %s",
			senderStyle.Render("[you]:"),
			input))
	}

	m.viewport.SetContent(strings.Join(m.messages, "\n"))
	m.viewport.GotoBottom()

	encoded, err := protocol.Encode(env)
	if err != nil {
		return
	}
	m.conn.Write(encoded)
}
