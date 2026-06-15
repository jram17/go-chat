package client

import (
	"crypto/sha256"
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

// Color palette
var (
	accent     = lipgloss.Color("#7C3AED")
	green      = lipgloss.Color("#10B981")
	yellow     = lipgloss.Color("#F59E0B")
	red        = lipgloss.Color("#EF4444")
	cyan       = lipgloss.Color("#06B6D4")
	dim        = lipgloss.Color("#6B7280")
	white      = lipgloss.Color("#F9FAFB")
	darkBorder = lipgloss.Color("#374151")
)

// Username color palette
var userColors = []lipgloss.Color{
	"#EF4444", "#F59E0B", "#10B981", "#06B6D4", "#3B82F6",
	"#8B5CF6", "#EC4899", "#F97316", "#14B8A6", "#A855F7",
}

// Commands for autocomplete
var commands = []struct {
	name string
	desc string
}{
	{"/msg", "Send encrypted DM"},
	{"/users", "List online users"},
	{"/help", "Show commands"},
	{"/quit", "Disconnect"},
}

// Styles
var (
	youStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(green)

	privateStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(yellow)

	privateSendStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(yellow)

	systemStyle = lipgloss.NewStyle().
			Foreground(dim).
			Italic(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(red).
			Bold(true)

	helpStyle = lipgloss.NewStyle().
			Foreground(cyan)

	helpCmdStyle = lipgloss.NewStyle().
			Foreground(cyan).
			Bold(true)

	helpDescStyle = lipgloss.NewStyle().
			Foreground(dim)

	timestampStyle = lipgloss.NewStyle().
			Foreground(dim)

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(white).
			Background(accent).
			Padding(0, 1).
			Width(80)

	inputBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(darkBorder).
				Padding(0, 1)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(dim).
			Padding(0, 1)

	autocompleteStyle = lipgloss.NewStyle().
				Foreground(cyan).
				Padding(0, 1)

	autocompleteActiveStyle = lipgloss.NewStyle().
				Foreground(white).
				Background(lipgloss.Color("#374151")).
				Padding(0, 1)

	bannerStyle = lipgloss.NewStyle().
			Foreground(accent).
			Bold(true)

	bannerDimStyle = lipgloss.NewStyle().
			Foreground(dim)

	typingStyle = lipgloss.NewStyle().
			Foreground(dim).
			Italic(true)
)

// IncomingMsg is sent from the network reader goroutine to the TUI
type IncomingMsg struct {
	Env protocol.Envelope
}

// ErrMsg is sent when the connection is lost
type ErrMsg struct {
	Err error
}

// typingExpiredMsg clears the typing indicator after timeout
type typingExpiredMsg struct {
	who string
}

type Model struct {
	viewport         viewport.Model
	textarea         textarea.Model
	messages         []string
	conn             net.Conn
	username         string
	privateKey       []byte
	peerKeys         map[string][]byte
	ready            bool
	width            int
	height           int
	quitting         bool
	showAutocomplete bool
	autocompleteIdx  int
	filteredCmds     []int
	typingUsers      map[string]time.Time
	lastTypingSent   time.Time
	typingTarget     string // who we're currently typing a /msg to
}

func NewModel(conn net.Conn, username string, privateKey []byte, peerKeys map[string][]byte) Model {
	ta := textarea.New()
	ta.Placeholder = "Type a message... (/help for commands)"
	ta.Focus()
	ta.SetHeight(1)
	ta.ShowLineNumbers = false
	ta.KeyMap.InsertNewline.SetEnabled(false)
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.CharLimit = 512

	vp := viewport.New(80, 20)

	return Model{
		viewport:    vp,
		textarea:    ta,
		messages:    []string{},
		conn:        conn,
		username:    username,
		privateKey:  privateKey,
		peerKeys:    peerKeys,
		typingUsers: make(map[string]time.Time),
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
		headerHeight := 1
		statusHeight := 1
		inputHeight := 3
		autocompleteHeight := 0
		if m.showAutocomplete {
			autocompleteHeight = len(m.filteredCmds) + 1
		}
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - headerHeight - statusHeight - inputHeight - autocompleteHeight - 2
		m.textarea.SetWidth(msg.Width - 4)
		headerStyle = headerStyle.Width(msg.Width)
		m.ready = true
		if len(m.messages) == 0 {
			m.showBanner()
		}
		m.viewport.SetContent(strings.Join(m.messages, "\n"))
		m.viewport.GotoBottom()

	case tea.KeyMsg:
		// Handle autocomplete navigation
		if m.showAutocomplete {
			switch msg.Type {
			case tea.KeyTab, tea.KeyDown:
				if len(m.filteredCmds) > 0 {
					m.autocompleteIdx = (m.autocompleteIdx + 1) % len(m.filteredCmds)
				}
				return m, nil
			case tea.KeyShiftTab, tea.KeyUp:
				if len(m.filteredCmds) > 0 {
					m.autocompleteIdx = (m.autocompleteIdx - 1 + len(m.filteredCmds)) % len(m.filteredCmds)
				}
				return m, nil
			case tea.KeyEnter:
				if len(m.filteredCmds) > 0 {
					cmd := commands[m.filteredCmds[m.autocompleteIdx]]
					m.textarea.Reset()
					m.textarea.SetValue(cmd.name + " ")
					m.showAutocomplete = false
					return m, nil
				}
			case tea.KeyEsc:
				m.showAutocomplete = false
				return m, nil
			}
		}

		switch msg.Type {
		case tea.KeyCtrlC:
			m.quitting = true
			return m, tea.Quit
		case tea.KeyEsc:
			if m.showAutocomplete {
				m.showAutocomplete = false
				return m, nil
			}
			m.quitting = true
			return m, tea.Quit
		case tea.KeyEnter:
			if m.showAutocomplete {
				break
			}
			input := m.textarea.Value()
			if input == "" {
				return m, nil
			}
			m.textarea.Reset()
			m.showAutocomplete = false
			m.typingTarget = ""
			if m.handleCommand(input) {
				m.viewport.SetContent(strings.Join(m.messages, "\n"))
				m.viewport.GotoBottom()
				if m.quitting {
					return m, tea.Quit
				}
				return m, nil
			}
			m.sendMessage(input)
			return m, nil
		default:
			// Send typing indicator for /msg
			m.maybeSendTyping()
		}

	case IncomingMsg:
		m.handleIncoming(msg.Env)
		m.viewport.SetContent(strings.Join(m.messages, "\n"))
		m.viewport.GotoBottom()

	case ErrMsg:
		m.appendMsg(errorStyle.Render(" ⚠  Disconnected from server"))
		m.viewport.SetContent(strings.Join(m.messages, "\n"))
		m.viewport.GotoBottom()

	case typingExpiredMsg:
		delete(m.typingUsers, msg.who)
	}

	m.viewport, vpCmd = m.viewport.Update(msg)
	m.textarea, taCmd = m.textarea.Update(msg)

	// Update autocomplete based on current input
	m.updateAutocomplete()

	return m, tea.Batch(vpCmd, taCmd)
}

func (m *Model) maybeSendTyping() {
	input := m.textarea.Value()
	if !strings.HasPrefix(input, "/msg ") {
		m.typingTarget = ""
		return
	}
	parts := strings.SplitN(input, " ", 3)
	if len(parts) < 2 {
		return
	}
	recipient := parts[1]
	if recipient == "" {
		return
	}

	// Throttle: only send typing once per 2 seconds
	if recipient == m.typingTarget && time.Since(m.lastTypingSent) < 2*time.Second {
		return
	}

	m.typingTarget = recipient
	m.lastTypingSent = time.Now()

	env := protocol.Envelope{
		Type:      protocol.MessageTypeTyping,
		From:      m.username,
		To:        recipient,
		Timestamp: time.Now().Unix(),
	}
	encoded, err := protocol.Encode(env)
	if err == nil {
		m.conn.Write(encoded)
	}
}

func (m *Model) updateAutocomplete() {
	input := m.textarea.Value()
	if strings.HasPrefix(input, "/") && !strings.Contains(input, " ") && len(input) > 0 {
		m.filteredCmds = nil
		for i, cmd := range commands {
			if strings.HasPrefix(cmd.name, input) {
				m.filteredCmds = append(m.filteredCmds, i)
			}
		}
		if len(m.filteredCmds) > 0 && input != commands[m.filteredCmds[0]].name {
			m.showAutocomplete = true
			if m.autocompleteIdx >= len(m.filteredCmds) {
				m.autocompleteIdx = 0
			}
		} else {
			m.showAutocomplete = false
		}
	} else {
		m.showAutocomplete = false
	}
}

func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	header := headerStyle.Render(fmt.Sprintf(" 🔒 go-chat • %s ", m.username))

	// Build typing indicator string
	typingStr := m.typingIndicator()

	status := statusBarStyle.Render(
		fmt.Sprintf("🟢 Connected • %d peers%s • /help for commands",
			len(m.peerKeys), typingStr))

	input := inputBorderStyle.Width(m.width - 2).Render(m.textarea.View())

	// Build autocomplete dropdown
	autocomplete := ""
	if m.showAutocomplete && len(m.filteredCmds) > 0 {
		var lines []string
		for i, idx := range m.filteredCmds {
			cmd := commands[idx]
			line := fmt.Sprintf(" %s  %s", cmd.name, helpDescStyle.Render(cmd.desc))
			if i == m.autocompleteIdx {
				lines = append(lines, autocompleteActiveStyle.Render(line))
			} else {
				lines = append(lines, autocompleteStyle.Render(line))
			}
		}
		autocomplete = strings.Join(lines, "\n") + "\n"
	}

	return fmt.Sprintf("%s\n%s\n%s\n%s%s",
		header,
		m.viewport.View(),
		status,
		autocomplete,
		input,
	)
}

func (m *Model) typingIndicator() string {
	// Clean expired entries (older than 3 seconds)
	now := time.Now()
	for user, t := range m.typingUsers {
		if now.Sub(t) > 3*time.Second {
			delete(m.typingUsers, user)
		}
	}

	if len(m.typingUsers) == 0 {
		return ""
	}

	var typers []string
	for user := range m.typingUsers {
		typers = append(typers, user)
	}

	if len(typers) == 1 {
		return typingStyle.Render(fmt.Sprintf(" • %s is typing...", typers[0]))
	}
	return typingStyle.Render(fmt.Sprintf(" • %s are typing...", strings.Join(typers, ", ")))
}

func (m *Model) showBanner() {
	m.appendMsg(bannerStyle.Render(""))
	m.appendMsg(bannerStyle.Render("   ┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓"))
	m.appendMsg(bannerStyle.Render("   ┃         🔒  g o - c h a t               ┃"))
	m.appendMsg(bannerStyle.Render("   ┃                                         ┃"))
	m.appendMsg(bannerDimStyle.Render("   ┃   End-to-end encrypted chat             ┃"))
	m.appendMsg(bannerDimStyle.Render("   ┃   X25519 + AES-256-GCM                 ┃"))
	m.appendMsg(bannerDimStyle.Render("   ┃                                         ┃"))
	m.appendMsg(bannerDimStyle.Render("   ┃   Type /help for commands               ┃"))
	m.appendMsg(bannerStyle.Render("   ┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛"))
	m.appendMsg("")
}

func (m *Model) appendMsg(msg string) {
	m.messages = append(m.messages, msg)
}

func (m *Model) timestamp() string {
	return timestampStyle.Render(time.Now().Format("15:04"))
}

func colorForUser(username string) lipgloss.Color {
	h := sha256.Sum256([]byte(username))
	idx := int(h[0]) % len(userColors)
	return userColors[idx]
}

func (m *Model) styledUsername(username string) string {
	color := colorForUser(username)
	return lipgloss.NewStyle().Bold(true).Foreground(color).Render(username)
}

func wrapText(text string, width int) string {
	if width <= 0 || len(text) <= width {
		return text
	}
	var result strings.Builder
	words := strings.Fields(text)
	lineLen := 0
	for i, word := range words {
		if i > 0 && lineLen+1+len(word) > width {
			result.WriteString("\n    ")
			lineLen = 4
		} else if i > 0 {
			result.WriteByte(' ')
			lineLen++
		}
		result.WriteString(word)
		lineLen += len(word)
	}
	return result.String()
}

// handleCommand processes slash commands. Returns true if input was a command.
func (m *Model) handleCommand(input string) bool {
	if !strings.HasPrefix(input, "/") {
		return false
	}

	cmd := strings.SplitN(input, " ", 2)[0]

	switch cmd {
	case "/help":
		m.appendMsg("")
		m.appendMsg(helpStyle.Render("  ╭──────────────────────────────────────────╮"))
		m.appendMsg(helpStyle.Render("  │          Available Commands              │"))
		m.appendMsg(helpStyle.Render("  ├──────────────────────────────────────────┤"))
		m.appendMsg(fmt.Sprintf("  │ %s  %s",
			helpCmdStyle.Render("/msg <user> <text>"),
			helpDescStyle.Render("Encrypted DM")))
		m.appendMsg(fmt.Sprintf("  │ %s              %s",
			helpCmdStyle.Render("/users"),
			helpDescStyle.Render("Online users")))
		m.appendMsg(fmt.Sprintf("  │ %s               %s",
			helpCmdStyle.Render("/help"),
			helpDescStyle.Render("This menu")))
		m.appendMsg(fmt.Sprintf("  │ %s               %s",
			helpCmdStyle.Render("/quit"),
			helpDescStyle.Render("Disconnect")))
		m.appendMsg(helpStyle.Render("  ╰──────────────────────────────────────────╯"))
		m.appendMsg("")
		return true

	case "/users":
		env := protocol.Envelope{
			Type:      protocol.MessageTypeUserList,
			From:      m.username,
			Timestamp: time.Now().Unix(),
		}
		encoded, err := protocol.Encode(env)
		if err == nil {
			m.conn.Write(encoded)
		}
		return true

	case "/quit":
		m.appendMsg(systemStyle.Render("  Disconnecting..."))
		m.quitting = true
		return true

	case "/msg":
		return false

	default:
		m.appendMsg(errorStyle.Render(fmt.Sprintf("  ⚠ Unknown command: %s (type /help)", cmd)))
		return true
	}
}

func (m *Model) handleIncoming(env protocol.Envelope) {
	ts := m.timestamp()
	wrapWidth := m.width - 20

	switch env.Type {
	case protocol.MessageTypeJoin:
		m.appendMsg(fmt.Sprintf("  %s %s",
			ts,
			systemStyle.Render(fmt.Sprintf("→ %s joined the chat", env.From))))

	case protocol.MessageTypeLeave:
		m.appendMsg(fmt.Sprintf("  %s %s",
			ts,
			systemStyle.Render(fmt.Sprintf("← %s left the chat", env.From))))

	case protocol.MessageTypeChat:
		content := wrapText(string(env.Payload), wrapWidth)
		m.appendMsg(fmt.Sprintf("  %s %s: %s",
			ts,
			m.styledUsername(env.From),
			content))

	case protocol.MessageTypePrivate:
		// Terminal bell for private message notification
		fmt.Print("\a")

		senderPub, ok := m.peerKeys[env.From]
		if !ok {
			m.appendMsg(errorStyle.Render("  ⚠ Sender public key not found"))
			return
		}
		key, err := crypto.ComputeSharedSecret(m.privateKey, senderPub)
		if err != nil {
			m.appendMsg(errorStyle.Render("  ⚠ Key derivation failed"))
			return
		}
		plaintext, err := crypto.Decrypt(key, env.Payload)
		if err != nil {
			m.appendMsg(errorStyle.Render("  ⚠ Decryption failed"))
			return
		}
		content := wrapText(string(plaintext), wrapWidth)
		m.appendMsg(fmt.Sprintf("  %s %s %s",
			ts,
			privateStyle.Render(fmt.Sprintf("🔒 %s:", env.From)),
			content))

	case protocol.MessageTypeKeyExchange:
		m.peerKeys[env.From] = env.Payload
		m.appendMsg(fmt.Sprintf("  %s %s",
			ts,
			systemStyle.Render(fmt.Sprintf("🔑 Key exchanged with %s", env.From))))

	case protocol.MessageTypeTyping:
		m.typingUsers[env.From] = time.Now()

	case protocol.MessageTypeUserList:
		users := strings.Split(string(env.Payload), ",")
		m.appendMsg("")
		m.appendMsg(helpStyle.Render(fmt.Sprintf("  ╭── Online Users (%d) ──╮", len(users))))
		for _, u := range users {
			if u == m.username {
				m.appendMsg(fmt.Sprintf("  │  ● %s %s",
					m.styledUsername(u),
					helpDescStyle.Render("(you)")))
			} else {
				m.appendMsg(fmt.Sprintf("  │  ○ %s", m.styledUsername(u)))
			}
		}
		m.appendMsg(helpStyle.Render("  ╰────────────────────────╯"))
		m.appendMsg("")
		m.viewport.SetContent(strings.Join(m.messages, "\n"))
		m.viewport.GotoBottom()
	}
}

func (m *Model) sendMessage(input string) {
	var env protocol.Envelope
	ts := m.timestamp()
	wrapWidth := m.width - 20

	if strings.HasPrefix(input, "/msg") {
		parts := strings.SplitN(input, " ", 3)
		if len(parts) < 3 {
			m.appendMsg(errorStyle.Render("  Usage: /msg <user> <message>"))
			m.viewport.SetContent(strings.Join(m.messages, "\n"))
			m.viewport.GotoBottom()
			return
		}
		recipient := parts[1]
		message := parts[2]

		recipientPub, ok := m.peerKeys[recipient]
		if !ok {
			m.appendMsg(errorStyle.Render(fmt.Sprintf("  ⚠ No key for %s — are they online?", recipient)))
			m.viewport.SetContent(strings.Join(m.messages, "\n"))
			m.viewport.GotoBottom()
			return
		}
		key, err := crypto.ComputeSharedSecret(m.privateKey, recipientPub)
		if err != nil {
			m.appendMsg(errorStyle.Render("  ⚠ Key derivation failed"))
			m.viewport.SetContent(strings.Join(m.messages, "\n"))
			m.viewport.GotoBottom()
			return
		}
		ciphertext, err := crypto.Encrypt(key, []byte(message))
		if err != nil {
			m.appendMsg(errorStyle.Render("  ⚠ Encryption failed"))
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
		content := wrapText(message, wrapWidth)
		m.appendMsg(fmt.Sprintf("  %s %s %s",
			ts,
			privateSendStyle.Render(fmt.Sprintf("🔒 → %s:", recipient)),
			content))
	} else {
		env = protocol.Envelope{
			Type:      protocol.MessageTypeChat,
			From:      m.username,
			Payload:   []byte(input),
			Timestamp: time.Now().Unix(),
		}
		content := wrapText(input, wrapWidth)
		m.appendMsg(fmt.Sprintf("  %s %s %s",
			ts,
			youStyle.Render("you:"),
			content))
	}

	m.viewport.SetContent(strings.Join(m.messages, "\n"))
	m.viewport.GotoBottom()

	encoded, err := protocol.Encode(env)
	if err != nil {
		return
	}
	m.conn.Write(encoded)
}
