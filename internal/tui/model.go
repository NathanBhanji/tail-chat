package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/NathanBhanji/tail-chat/internal/chat"
	"github.com/NathanBhanji/tail-chat/internal/discovery"
	"github.com/NathanBhanji/tail-chat/internal/protocol"
)

// View represents the current screen.
type View int

const (
	ViewPeers View = iota
	ViewChat
	ViewGroupCreate
	ViewSearch
)

// IncomingMsg is sent when a new chat message arrives.
type IncomingMsg struct {
	ChatKey string
	Message chat.Message
}

// PeersUpdatedMsg is sent when the peer list changes.
type PeersUpdatedMsg struct {
	Peers []discovery.Peer
}

// ConnectedMsg is sent when a connection to a peer is established.
type ConnectedMsg struct {
	Hostname string
	Err      error
}

// GroupInviteMsg is sent when we receive a group invite.
type GroupInviteMsg struct {
	Invite *protocol.GroupInvite
	From   string
}

// PeerConnectMsg is sent when an inbound peer connects to us.
type PeerConnectMsg struct {
	Hostname string
}

// MessageSentMsg is sent after an async send completes.
type MessageSentMsg struct {
	ChatKey string
	Err     error
}

// ErrorMsg is a transient error to display.
type ErrorMsg struct {
	Err error
}

// TickMsg is sent periodically for UI updates.
type TickMsg time.Time

// TypingMsg is sent when a peer starts or stops typing.
type TypingMsg struct {
	ChatKey  string
	IsTyping bool
}

// ReactionUpdatedMsg is sent when a reaction is added or removed.
type ReactionUpdatedMsg struct {
	ChatKey string
}

// StatusUpdatedMsg is sent when a peer changes their status.
type StatusUpdatedMsg struct {
	Hostname string
	State    string
}

// FileSentMsg is sent after an async file send starts (contains the message ID for tracking).
type FileSentMsg struct {
	ChatKey string
	MsgID   string
	Err     error
}

// searchResult holds one search hit.
type searchResult struct {
	ChatKey string
	Message chat.Message
}

// Model is the main bubbletea model.
type Model struct {
	// Deps
	chatMgr     *chat.Manager
	peerWatcher *discovery.Watcher

	// State
	view           View
	peers          []discovery.Peer
	peerCursor     int
	activeChatKey  string // hostname or "group:<id>"
	messages       []chat.Message
	width          int
	height         int
	err            string
	errExpiry      time.Time
	connecting     string // hostname we're currently connecting to
	connectingDots int    // animation counter

	// Group creation
	groupName     string
	groupSelected map[string]bool // hostname -> selected
	groupCursor   int

	// Scrollback
	scrollOffset int

	// Typing indicators
	lastTypingSent time.Time

	// Emoji tab completion
	emojiCompletions []string
	emojiCompIdx     int
	emojiReplaceFrom int // cursor position where replacement starts

	// Search
	searchResults []searchResult
	searchCursor  int
	searchDone    bool

	// Vim gg detection
	lastKey     string
	lastKeyTime time.Time

	// Components
	input textinput.Model
}

// NewModel creates the initial TUI model.
func NewModel(chatMgr *chat.Manager, watcher *discovery.Watcher) Model {
	ti := textinput.New()
	ti.Placeholder = "Type a message..."
	ti.CharLimit = 4096
	ti.Width = 60

	return Model{
		chatMgr:     chatMgr,
		peerWatcher: watcher,
		view:        ViewPeers,
		peers:       watcher.Peers(),
		input:       ti,
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, tickCmd())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.Width = msg.Width - 8
		return m, nil

	case IncomingMsg:
		if m.view == ViewChat && msg.ChatKey == m.activeChatKey {
			m.messages = m.chatMgr.GetMessages(m.activeChatKey)
			return m, nil
		}
		// Message arrived for a chat we're not viewing — notify
		return m, notifyCmd(msg.Message.Sender, msg.Message.Content)

	case PeerConnectMsg:
		return m, nil

	case NotifyMsg:
		return m, nil

	case MessageSentMsg:
		if msg.Err != nil {
			m.err = msg.Err.Error()
			m.errExpiry = time.Now().Add(3 * time.Second)
		}
		if m.view == ViewChat && msg.ChatKey == m.activeChatKey {
			m.messages = m.chatMgr.GetMessages(m.activeChatKey)
		}
		return m, nil

	case FileSentMsg:
		if msg.Err != nil {
			m.err = msg.Err.Error()
			m.errExpiry = time.Now().Add(3 * time.Second)
		}
		if m.view == ViewChat && msg.ChatKey == m.activeChatKey {
			m.messages = m.chatMgr.GetMessages(m.activeChatKey)
		}
		return m, nil

	case PeersUpdatedMsg:
		m.peers = msg.Peers
		return m, nil

	case ConnectedMsg:
		m.connecting = ""
		if msg.Err != nil {
			m.err = fmt.Sprintf("Connection to %s failed: peer may not be running tailchat", msg.Hostname)
			m.errExpiry = time.Now().Add(5 * time.Second)
		} else {
			m.err = ""
			m = m.openChat(msg.Hostname)
		}
		return m, nil

	case GroupInviteMsg:
		// Auto-accept: groups appear instantly on the peer list
		m.chatMgr.AcceptGroupInvite(msg.Invite, msg.From)
		return m, nil

	case TypingMsg:
		// Just triggers a re-render
		return m, nil

	case ReactionUpdatedMsg:
		if m.view == ViewChat && msg.ChatKey == m.activeChatKey {
			m.messages = m.chatMgr.GetMessages(m.activeChatKey)
		}
		return m, nil

	case StatusUpdatedMsg:
		// Just triggers a re-render (peer list shows status)
		return m, nil

	case ErrorMsg:
		m.err = msg.Err.Error()
		m.errExpiry = time.Now().Add(5 * time.Second)
		return m, nil

	case TickMsg:
		// Clear expired errors
		if m.err != "" && time.Now().After(m.errExpiry) {
			m.err = ""
		}
		// Animate connecting dots
		if m.connecting != "" {
			m.connectingDots = (m.connectingDots + 1) % 4
		}
		// Refresh peers
		m.peers = m.peerWatcher.Peers()
		// Defensive refresh: re-read messages when in chat view
		if m.view == ViewChat && m.activeChatKey != "" {
			m.messages = m.chatMgr.GetMessages(m.activeChatKey)
		}
		return m, tickCmd()
	}

	// Update text input
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// openChat sets up the model to display a chat.
func (m Model) openChat(chatKey string) Model {
	m.activeChatKey = chatKey
	m.chatMgr.LoadMessages(chatKey)
	m.messages = m.chatMgr.GetMessages(chatKey)
	m.chatMgr.ClearUnread(chatKey)
	m.chatMgr.SendReadReceipts(chatKey)
	m.view = ViewChat
	m.scrollOffset = 0
	m.emojiCompletions = nil
	m.emojiCompIdx = 0
	m.input.Placeholder = "Type a message... (:emoji: tab \u2022 /file path)"
	m.input.SetValue("")
	m.input.Focus()
	return m
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Global quit
	if key == "ctrl+c" {
		return m, tea.Quit
	}

	switch m.view {
	case ViewPeers:
		return m.handlePeerKeys(key)
	case ViewChat:
		return m.handleChatKeys(msg)
	case ViewGroupCreate:
		return m.handleGroupCreateKeys(msg)
	case ViewSearch:
		return m.handleSearchKeys(msg)
	}

	return m, nil
}

func (m Model) handlePeerKeys(key string) (tea.Model, tea.Cmd) {
	totalItems := len(m.peers) + len(m.chatMgr.Groups())

	switch key {
	case "q", "ctrl+d":
		return m, tea.Quit

	case "up", "k":
		if m.peerCursor > 0 {
			m.peerCursor--
		}

	case "down", "j":
		if m.peerCursor < totalItems-1 {
			m.peerCursor++
		}

	case "enter", "l":
		return m.selectPeerItem()

	case "g":
		// Vim gg: go to top (double-tap g)
		if m.lastKey == "g" && time.Since(m.lastKeyTime) < 500*time.Millisecond {
			m.peerCursor = 0
			m.lastKey = ""
			return m, nil
		}
		m.lastKey = "g"
		m.lastKeyTime = time.Now()
		return m, nil

	case "G":
		// Vim G: go to bottom
		if totalItems > 0 {
			m.peerCursor = totalItems - 1
		}

	case "/":
		// Enter search mode
		m.view = ViewSearch
		m.searchResults = nil
		m.searchCursor = 0
		m.searchDone = false
		m.input.SetValue("")
		m.input.Placeholder = "Search messages..."
		m.input.Focus()
		return m, nil

	case "s":
		// Cycle status: available -> away -> busy -> dnd -> available
		current := m.chatMgr.GetStatus(m.peerWatcher.SelfHostname())
		states := []string{"available", "away", "busy", "dnd"}
		idx := 0
		for i, s := range states {
			if s == current {
				idx = i
				break
			}
		}
		next := states[(idx+1)%len(states)]
		m.chatMgr.SetStatus(next)

	case "n":
		// Create group
		m.view = ViewGroupCreate
		m.groupName = ""
		m.groupSelected = make(map[string]bool)
		m.groupCursor = 0
		m.input.SetValue("")
		m.input.Placeholder = "Group name..."
		m.input.Focus()
		return m, nil

	case "r":
		// Refresh peers
		m.peers = m.peerWatcher.Peers()
	}

	// Reset gg detection for non-g keys
	if key != "g" {
		m.lastKey = key
		m.lastKeyTime = time.Now()
	}

	return m, nil
}

func (m Model) selectPeerItem() (tea.Model, tea.Cmd) {
	groups := m.chatMgr.Groups()

	// Check if cursor is on a peer
	if m.peerCursor < len(m.peers) {
		peer := m.peers[m.peerCursor]
		if !peer.Online {
			m.err = fmt.Sprintf("%s is offline", peer.Hostname)
			m.errExpiry = time.Now().Add(3 * time.Second)
			return m, nil
		}

		// Check if already connected
		if m.chatMgr.IsConnected(peer.Hostname) {
			m = m.openChat(peer.Hostname)
			return m, nil
		}

		// Don't double-connect
		if m.connecting != "" {
			return m, nil
		}

		// Connect (async — UI stays responsive)
		ip := peer.TailscaleIP
		hostname := peer.Hostname
		m.connecting = hostname
		m.connectingDots = 0
		return m, func() tea.Msg {
			_, err := m.chatMgr.ConnectToPeer(ip)
			return ConnectedMsg{Hostname: hostname, Err: err}
		}
	}

	// Check if cursor is on a group
	groupIdx := m.peerCursor - len(m.peers)
	if groupIdx >= 0 && groupIdx < len(groups) {
		group := groups[groupIdx]
		m = m.openChat("group:" + group.ID)
		return m, nil
	}

	return m, nil
}

func (m Model) handleChatKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "esc":
		m.view = ViewPeers
		m.input.Blur()
		m.chatMgr.SendTyping(m.activeChatKey, false)
		return m, nil

	case "ctrl+u":
		// Vim: scroll up half page
		half := (m.height - 8) / 2
		if half < 1 {
			half = 1
		}
		m.scrollOffset += half
		maxScroll := len(m.messages) - 1
		if maxScroll < 0 {
			maxScroll = 0
		}
		if m.scrollOffset > maxScroll {
			m.scrollOffset = maxScroll
		}
		return m, nil

	case "ctrl+d":
		// Vim: scroll down half page
		half := (m.height - 8) / 2
		if half < 1 {
			half = 1
		}
		m.scrollOffset -= half
		if m.scrollOffset < 0 {
			m.scrollOffset = 0
		}
		return m, nil

	case "pgup":
		avail := m.height - 8
		if avail < 5 {
			avail = 5
		}
		m.scrollOffset += avail
		maxScroll := len(m.messages) - 1
		if maxScroll < 0 {
			maxScroll = 0
		}
		if m.scrollOffset > maxScroll {
			m.scrollOffset = maxScroll
		}
		return m, nil

	case "pgdown":
		avail := m.height - 8
		if avail < 5 {
			avail = 5
		}
		m.scrollOffset -= avail
		if m.scrollOffset < 0 {
			m.scrollOffset = 0
		}
		return m, nil

	case "tab":
		// Emoji tab completion
		return m.handleEmojiCompletion()

	case "enter":
		content := strings.TrimSpace(m.input.Value())
		if content == "" {
			return m, nil
		}
		m.input.SetValue("")
		m.emojiCompletions = nil
		m.emojiCompIdx = 0
		m.scrollOffset = 0
		m.chatMgr.SendTyping(m.activeChatKey, false)

		// Handle /file command (sends via Taildrop)
		if strings.HasPrefix(content, "/file ") {
			path := strings.TrimSpace(content[6:])
			if strings.HasPrefix(m.activeChatKey, "group:") {
				m.err = "File transfer not supported in groups"
				m.errExpiry = time.Now().Add(3 * time.Second)
				return m, nil
			}
			chatKey := m.activeChatKey
			chatMgr := m.chatMgr
			return m, func() tea.Msg {
				msgID, err := chatMgr.SendFile(chatKey, path)
				return FileSentMsg{ChatKey: chatKey, MsgID: msgID, Err: err}
			}
		}

		// Handle /react command
		if strings.HasPrefix(content, "/react ") {
			emoji := strings.TrimSpace(content[7:])
			emoji = expandEmoji(emoji)
			// React to the last non-own message
			for i := len(m.messages) - 1; i >= 0; i-- {
				if !m.messages[i].IsOwn && m.messages[i].Sender != "system" {
					m.chatMgr.SendReaction(m.activeChatKey, m.messages[i].ID, emoji)
					break
				}
			}
			return m, nil
		}

		// Handle /status command
		if strings.HasPrefix(content, "/status ") {
			state := strings.TrimSpace(content[8:])
			m.chatMgr.SetStatus(state)
			return m, nil
		}

		// Normal message — expand emoji shortcodes before sending
		content = expandEmoji(content)

		// Send async so the TCP write doesn't block the UI
		chatKey := m.activeChatKey
		chatMgr := m.chatMgr
		return m, func() tea.Msg {
			var err error
			if strings.HasPrefix(chatKey, "group:") {
				groupID := strings.TrimPrefix(chatKey, "group:")
				err = chatMgr.SendGroupMessage(groupID, content)
			} else {
				err = chatMgr.SendMessage(chatKey, content)
			}
			return MessageSentMsg{ChatKey: chatKey, Err: err}
		}
	}

	// Clear emoji completions on any non-tab key
	m.emojiCompletions = nil
	m.emojiCompIdx = 0

	// Pass to text input
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)

	// Send typing indicator (throttled to once every 2s)
	if m.input.Value() != "" && time.Since(m.lastTypingSent) > 2*time.Second {
		m.lastTypingSent = time.Now()
		m.chatMgr.SendTyping(m.activeChatKey, true)
	}

	return m, cmd
}

func (m Model) handleEmojiCompletion() (tea.Model, tea.Cmd) {
	val := m.input.Value()

	// If already cycling through completions, advance to next
	if len(m.emojiCompletions) > 0 {
		m.emojiCompIdx = (m.emojiCompIdx + 1) % len(m.emojiCompletions)
		code := m.emojiCompletions[m.emojiCompIdx]
		newVal := val[:m.emojiReplaceFrom] + ":" + code + ":"
		m.input.SetValue(newVal)
		m.input.CursorEnd()
		return m, nil
	}

	// Find partial shortcode: last ':' followed by letters (no spaces)
	lastColon := strings.LastIndexByte(val, ':')
	if lastColon < 0 || lastColon >= len(val)-1 {
		return m, nil
	}
	prefix := val[lastColon+1:]
	if strings.ContainsAny(prefix, " \t") || len(prefix) == 0 {
		return m, nil
	}

	// Build matches
	lower := strings.ToLower(prefix)
	var matches []string
	for code := range shortcodes {
		if strings.HasPrefix(code, lower) {
			matches = append(matches, code)
		}
	}
	if len(matches) == 0 {
		return m, nil
	}
	sort.Strings(matches)

	m.emojiCompletions = matches
	m.emojiCompIdx = 0
	m.emojiReplaceFrom = lastColon

	code := matches[0]
	newVal := val[:lastColon] + ":" + code + ":"
	m.input.SetValue(newVal)
	m.input.CursorEnd()

	return m, nil
}

func (m Model) handleGroupCreateKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Phase 1: entering group name (text input active)
	if m.groupName == "" {
		switch key {
		case "esc":
			m.view = ViewPeers
			m.input.Blur()
			return m, nil
		case "enter":
			val := strings.TrimSpace(m.input.Value())
			if val == "" {
				return m, nil
			}
			m.groupName = val
			m.input.SetValue("")
			m.input.Blur()
			m.groupCursor = 0
			return m, nil
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	// Phase 2: selecting members from peer list
	connectedPeers := m.connectedPeers()

	switch key {
	case "esc":
		m.view = ViewPeers
		return m, nil

	case "up", "k":
		if m.groupCursor > 0 {
			m.groupCursor--
		}

	case "down", "j":
		if m.groupCursor < len(connectedPeers)-1 {
			m.groupCursor++
		}

	case " ", "enter":
		// Toggle selection
		if m.groupCursor < len(connectedPeers) {
			host := connectedPeers[m.groupCursor]
			m.groupSelected[host] = !m.groupSelected[host]
		}

	case "ctrl+s":
		// Create the group with selected members
		var members []string
		for host, sel := range m.groupSelected {
			if sel {
				members = append(members, host)
			}
		}
		if len(members) > 0 {
			m.chatMgr.CreateGroup(m.groupName, members)
			m.view = ViewPeers
		}
		return m, nil
	}

	return m, nil
}

// connectedPeers returns hostnames of all peers we have an active connection to.
func (m Model) connectedPeers() []string {
	var result []string
	for _, p := range m.peers {
		if m.chatMgr.IsConnected(p.Hostname) {
			result = append(result, p.Hostname)
		}
	}
	return result
}

func (m Model) handleSearchKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "esc":
		m.view = ViewPeers
		m.input.Blur()
		m.searchResults = nil
		return m, nil

	case "enter":
		if !m.searchDone {
			// Execute search
			query := strings.TrimSpace(m.input.Value())
			if query == "" {
				return m, nil
			}
			results := m.chatMgr.SearchMessages(query)
			m.searchResults = nil
			for chatKey, msgs := range results {
				for _, msg := range msgs {
					m.searchResults = append(m.searchResults, searchResult{
						ChatKey: chatKey,
						Message: msg,
					})
				}
			}
			// Sort by timestamp descending (newest first)
			sort.Slice(m.searchResults, func(i, j int) bool {
				return m.searchResults[i].Message.Timestamp.After(m.searchResults[j].Message.Timestamp)
			})
			m.searchDone = true
			m.searchCursor = 0
			m.input.Blur()
			return m, nil
		}

		// Select search result — jump to that chat
		if m.searchCursor < len(m.searchResults) {
			result := m.searchResults[m.searchCursor]
			m.searchResults = nil
			m = m.openChat(result.ChatKey)
		}
		return m, nil

	case "up", "k":
		if m.searchDone && m.searchCursor > 0 {
			m.searchCursor--
		}
		return m, nil

	case "down", "j":
		if m.searchDone && m.searchCursor < len(m.searchResults)-1 {
			m.searchCursor++
		}
		return m, nil

	case "/":
		// Start a new search
		if m.searchDone {
			m.searchDone = false
			m.searchResults = nil
			m.input.SetValue("")
			m.input.Focus()
			return m, nil
		}
	}

	// Pass to text input when in query mode
	if !m.searchDone {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	switch m.view {
	case ViewPeers:
		return m.viewPeers()
	case ViewChat:
		return m.viewChat()
	case ViewGroupCreate:
		return m.viewGroupCreate()
	case ViewSearch:
		return m.viewSearch()
	}

	return ""
}

func (m Model) viewPeers() string {
	var b strings.Builder

	// Title
	title := titleStyle.Render(" tailchat ")
	b.WriteString(title)
	b.WriteString("\n\n")

	// Self info with status
	selfHost := m.peerWatcher.SelfHostname()
	selfIP := m.peerWatcher.SelfIP()
	selfStatus := m.chatMgr.GetStatus(selfHost)
	statusLabel := ""
	if selfStatus != "" && selfStatus != "available" {
		statusLabel = " " + statusText(selfStatus)
	}
	b.WriteString(fmt.Sprintf("  %s  %s%s\n",
		peerOnline.Render("\u25cf"),
		lipgloss.NewStyle().Foreground(text).Bold(true).Render(
			fmt.Sprintf("%s (%s)", selfHost, selfIP)),
		statusLabel,
	))
	b.WriteString("\n")

	// Peers section
	onlineCount := 0
	for _, p := range m.peers {
		if p.Online {
			onlineCount++
		}
	}

	b.WriteString(sidebarTitle.Render(fmt.Sprintf("  Peers (%d online)", onlineCount)))
	b.WriteString("\n")

	idx := 0
	for _, peer := range m.peers {
		var dot string
		switch {
		case m.chatMgr.IsConnected(peer.Hostname):
			dot = encryptedBadge.Render("\u25cf") // green — connected + encrypted
		case peer.RunningTailchat:
			dot = tailchatOnline.Render("\u25cf") // cyan — running tailchat
		case peer.Online:
			dot = peerOnline.Render("\u25cb") // green outline — online on tailscale only
		default:
			dot = peerOffline.Render("\u25cb") // gray — offline
		}

		connStatus := ""
		if m.chatMgr.IsConnected(peer.Hostname) {
			connStatus = encryptedBadge.Render(" \U0001f512")
			// Show peer status if not available
			state := m.chatMgr.GetStatus(peer.Hostname)
			if state != "" && state != "available" {
				connStatus += " " + statusText(state)
			}
		}

		unreadBadge := ""
		if n := m.chatMgr.Unread(peer.Hostname); n > 0 {
			unreadBadge = unreadStyle.Render(fmt.Sprintf(" (%d)", n))
		}

		tcBadge := ""
		if peer.RunningTailchat && !m.chatMgr.IsConnected(peer.Hostname) {
			tcBadge = tailchatBadge.Render(" [tailchat]")
		}

		name := fmt.Sprintf("%s %s  %s%s%s%s", dot, peer.Hostname, helpStyle.Render(peer.TailscaleIP), connStatus, tcBadge, unreadBadge)

		if idx == m.peerCursor {
			b.WriteString(peerSelected.Render(name))
		} else {
			b.WriteString(peerNormal.Render(name))
		}
		b.WriteString("\n")
		idx++
	}

	// Groups section
	groups := m.chatMgr.Groups()
	if len(groups) > 0 {
		b.WriteString("\n")
		b.WriteString(sidebarTitle.Render("  Groups"))
		b.WriteString("\n")

		for _, g := range groups {
			name := fmt.Sprintf("%s %s (%d members)", groupBadge.Render("#"), g.Name, len(g.Members))
			if idx == m.peerCursor {
				b.WriteString(peerSelected.Render(name))
			} else {
				b.WriteString(peerNormal.Render(name))
			}
			b.WriteString("\n")
			idx++
		}
	}

	// Connecting indicator
	if m.connecting != "" {
		dots := strings.Repeat(".", m.connectingDots+1)
		b.WriteString("\n")
		b.WriteString(connectingStyle.Render(fmt.Sprintf("  Connecting to %s%s", m.connecting, dots)))
		b.WriteString("\n")
	}

	// Error
	if m.err != "" {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render("  " + m.err))
		b.WriteString("\n")
	}

	// Help
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("  j/k navigate \u2022 l/enter connect \u2022 gg/G top/bottom \u2022 / search \u2022 n new group"))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("  s cycle status \u2022 r refresh \u2022 q quit"))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render(fmt.Sprintf("  %s connected  %s tailchat  %s online  %s offline",
		encryptedBadge.Render("\u25cf"), tailchatOnline.Render("\u25cf"), peerOnline.Render("\u25cb"), peerOffline.Render("\u25cb"))))

	return appStyle.Render(b.String())
}

func (m Model) viewChat() string {
	var b strings.Builder

	// Header
	chatName := m.activeChatKey
	if strings.HasPrefix(chatName, "group:") {
		groups := m.chatMgr.Groups()
		for _, g := range groups {
			if "group:"+g.ID == m.activeChatKey {
				chatName = "# " + g.Name
				break
			}
		}
	}

	// Show peer status in header for 1:1 chats
	peerStatus := ""
	if !strings.HasPrefix(m.activeChatKey, "group:") {
		state := m.chatMgr.GetStatus(m.activeChatKey)
		if state != "" && state != "available" {
			peerStatus = " " + statusText(state)
		}
	}

	header := fmt.Sprintf(" tailchat \u2014 %s%s %s",
		chatName, peerStatus,
		encryptedBadge.Render("e2e encrypted"),
	)
	b.WriteString(titleStyle.Render(header))
	b.WriteString("\n\n")

	// Calculate extra lines below messages
	extraLines := 0
	isTyping := !strings.HasPrefix(m.activeChatKey, "group:") && m.chatMgr.IsTyping(m.activeChatKey)
	if isTyping {
		extraLines++
	}
	if m.scrollOffset > 0 {
		extraLines++
	}
	if m.err != "" {
		extraLines++
	}
	if len(m.emojiCompletions) > 0 {
		extraLines++
	}

	availHeight := m.height - 8 - extraLines
	if availHeight < 3 {
		availHeight = 3
	}

	// Determine visible message window with scrollback
	msgs := m.messages
	endIdx := len(msgs) - m.scrollOffset
	if endIdx > len(msgs) {
		endIdx = len(msgs)
	}
	if endIdx < 0 {
		endIdx = 0
	}

	// Work backwards from endIdx to find how many messages fit
	linesUsed := 0
	startIdx := endIdx
	for i := endIdx - 1; i >= 0; i-- {
		lines := 1 // the message line itself
		if len(msgs[i].Reactions) > 0 {
			lines++ // reaction line
		}
		if linesUsed+lines > availHeight {
			break
		}
		linesUsed += lines
		startIdx = i
	}

	// Render messages
	for i := startIdx; i < endIdx; i++ {
		msg := msgs[i]
		ts := msgTimeStyle.Render(msg.Timestamp.Format("15:04"))

		if msg.Sender == "system" {
			b.WriteString(fmt.Sprintf("  %s %s\n",
				ts, systemMsgStyle.Render(msg.Content)))
			continue
		}

		// File transfer messages get special rendering
		if msg.FileInfo != nil {
			var sender string
			if msg.IsOwn {
				sender = ownMsgStyle.Render("you")
			} else {
				sender = peerMsgStyle.Render(msg.Sender)
			}
			b.WriteString(fmt.Sprintf("  %s %s: %s\n",
				ts, sender, renderFileTransfer(msg.FileInfo)))
			continue
		}

		var sender string
		if msg.IsOwn {
			sender = ownMsgStyle.Render("you")
		} else {
			sender = peerMsgStyle.Render(msg.Sender)
		}

		// Delivery state for own messages
		tick := ""
		if msg.IsOwn {
			switch msg.State {
			case chat.StateSending:
				tick = deliveryPending.Render(" \u25cb")
			case chat.StateDelivered:
				tick = deliveryStyle.Render(" \u2713")
			case chat.StateRead:
				tick = deliveryStyle.Render(" \u2713\u2713")
			}
		}

		// Render content with URL/GIF detection
		rendered := renderContent(msg.Content)
		b.WriteString(fmt.Sprintf("  %s %s: %s%s\n",
			ts, sender, rendered, tick))

		// Reactions
		if len(msg.Reactions) > 0 {
			b.WriteString(fmt.Sprintf("  %s %s\n",
				strings.Repeat(" ", 5),
				renderReactions(msg.Reactions)))
		}
	}

	if len(msgs) == 0 {
		b.WriteString(helpStyle.Render("  No messages yet. Say hello!\n"))
		linesUsed = 1
	}

	// Pad to fill remaining space
	for i := linesUsed; i < availHeight; i++ {
		b.WriteString("\n")
	}

	// Scroll indicator
	if m.scrollOffset > 0 {
		b.WriteString(scrollIndicator.Render(fmt.Sprintf("  \u2191 %d older messages", m.scrollOffset)))
		b.WriteString("\n")
	}

	// Typing indicator
	if isTyping {
		b.WriteString(typingStyle.Render(fmt.Sprintf("  %s is typing...", m.activeChatKey)))
		b.WriteString("\n")
	}

	// Error in chat view
	if m.err != "" {
		b.WriteString(errorStyle.Render("  " + m.err))
		b.WriteString("\n")
	}

	// Emoji completion suggestions
	if len(m.emojiCompletions) > 0 {
		maxShow := 8
		comps := m.emojiCompletions
		if len(comps) > maxShow {
			comps = comps[:maxShow]
		}
		var parts []string
		for i, code := range comps {
			emoji := shortcodes[code]
			if i == m.emojiCompIdx && m.emojiCompIdx < len(comps) {
				parts = append(parts, completionSelected.Render(fmt.Sprintf(" %s :%s: ", emoji, code)))
			} else {
				parts = append(parts, completionStyle.Render(fmt.Sprintf(" %s :%s: ", emoji, code)))
			}
		}
		if len(m.emojiCompletions) > maxShow {
			parts = append(parts, helpStyle.Render(fmt.Sprintf("+%d more", len(m.emojiCompletions)-maxShow)))
		}
		b.WriteString("  " + strings.Join(parts, ""))
		b.WriteString("\n")
	}

	// Input
	b.WriteString("\n")
	b.WriteString(inputStyle.Render(m.input.View()))
	b.WriteString("\n")

	// Help
	b.WriteString(helpStyle.Render("  enter send \u2022 tab :emoji: \u2022 ctrl+u/d scroll \u2022 /react /file (taildrop) \u2022 esc back"))

	return appStyle.Render(b.String())
}

func (m Model) viewGroupCreate() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render(" tailchat \u2014 New Group "))
	b.WriteString("\n\n")

	if m.groupName == "" {
		// Phase 1: name entry
		b.WriteString("  Enter group name:\n\n")
		b.WriteString(inputStyle.Render(m.input.View()))
		b.WriteString("\n\n")
		b.WriteString(helpStyle.Render("  enter confirm \u2022 esc cancel"))
	} else {
		// Phase 2: member selection
		b.WriteString(fmt.Sprintf("  Group: %s\n", groupBadge.Render("# "+m.groupName)))
		b.WriteString("\n")
		b.WriteString(sidebarTitle.Render("  Select members"))
		b.WriteString("\n")

		connectedPeers := m.connectedPeers()
		if len(connectedPeers) == 0 {
			b.WriteString(helpStyle.Render("  No connected peers. Connect to peers first.\n"))
		}

		for i, host := range connectedPeers {
			check := peerOffline.Render("[ ]")
			if m.groupSelected[host] {
				check = encryptedBadge.Render("[x]")
			}

			line := fmt.Sprintf("%s %s", check, host)
			if i == m.groupCursor {
				b.WriteString(peerSelected.Render(line))
			} else {
				b.WriteString(peerNormal.Render(line))
			}
			b.WriteString("\n")
		}

		// Count selected
		selected := 0
		for _, sel := range m.groupSelected {
			if sel {
				selected++
			}
		}
		if selected > 0 {
			b.WriteString(fmt.Sprintf("\n  %s\n",
				encryptedBadge.Render(fmt.Sprintf("%d selected", selected))))
		}

		b.WriteString("\n")
		b.WriteString(helpStyle.Render("  j/k navigate \u2022 space toggle \u2022 ctrl+s create \u2022 esc cancel"))
	}

	return appStyle.Render(b.String())
}

func (m Model) viewSearch() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render(" tailchat \u2014 Search "))
	b.WriteString("\n\n")

	if !m.searchDone {
		b.WriteString("  Enter search query:\n\n")
		b.WriteString(inputStyle.Render(m.input.View()))
		b.WriteString("\n\n")
		b.WriteString(helpStyle.Render("  enter search \u2022 esc cancel"))
	} else {
		b.WriteString(fmt.Sprintf("  Results: %d matches\n\n", len(m.searchResults)))

		maxShow := m.height - 10
		if maxShow < 5 {
			maxShow = 5
		}
		if maxShow > len(m.searchResults) {
			maxShow = len(m.searchResults)
		}

		for i := 0; i < maxShow; i++ {
			r := m.searchResults[i]
			ts := r.Message.Timestamp.Format("Jan 02 15:04")
			line := fmt.Sprintf("  [%s] %s \u2014 %s: %s",
				helpStyle.Render(ts),
				sidebarTitle.Render(r.ChatKey),
				peerMsgStyle.Render(r.Message.Sender),
				truncate(r.Message.Content, 50),
			)
			if i == m.searchCursor {
				b.WriteString(peerSelected.Render(line))
			} else {
				b.WriteString(peerNormal.Render(line))
			}
			b.WriteString("\n")
		}

		if len(m.searchResults) == 0 {
			b.WriteString(helpStyle.Render("  No results found.\n"))
		}

		b.WriteString("\n")
		b.WriteString(helpStyle.Render("  j/k navigate \u2022 enter open chat \u2022 / new search \u2022 esc back"))
	}

	return appStyle.Render(b.String())
}

// --- Helpers ---

func statusText(state string) string {
	switch state {
	case "away":
		return statusAway.Render("\u25d0 away")
	case "busy":
		return statusBusy.Render("\u25cf busy")
	case "dnd":
		return statusDnd.Render("\u26d4 dnd")
	default:
		return ""
	}
}

func renderReactions(reactions []chat.Reaction) string {
	counts := make(map[string]int)
	var order []string
	for _, r := range reactions {
		if counts[r.Emoji] == 0 {
			order = append(order, r.Emoji)
		}
		counts[r.Emoji]++
	}
	var parts []string
	for _, emoji := range order {
		n := counts[emoji]
		if n > 1 {
			parts = append(parts, reactionStyle.Render(fmt.Sprintf("%s%d", emoji, n)))
		} else {
			parts = append(parts, reactionStyle.Render(emoji))
		}
	}
	return strings.Join(parts, " ")
}

func renderFileTransfer(fi *chat.FileInfo) string {
	size := formatFileSize(fi.Size)
	switch fi.State {
	case chat.FileSending:
		return fileIcon.Render("\U0001f4e4") + " " +
			fileSending.Render(fmt.Sprintf("%s (%s) sending...", fi.Filename, size)) +
			deliveryPending.Render(" \u25cb")
	case chat.FileSent:
		return fileIcon.Render("\U0001f4e4") + " " +
			fileSent.Render(fmt.Sprintf("%s (%s) sent", fi.Filename, size)) +
			deliveryStyle.Render(" \u2713")
	case chat.FileFailed:
		errMsg := fi.Error
		if errMsg == "" {
			errMsg = "unknown error"
		}
		return fileIcon.Render("\U0001f4e4") + " " +
			fileFailed.Render(fmt.Sprintf("%s failed: %s", fi.Filename, errMsg)) +
			errorStyle.Render(" \u2717")
	case chat.FileReceived:
		path := fi.Path
		if path == "" {
			path = fi.Filename
		}
		return fileIcon.Render("\U0001f4e5") + " " +
			fileReceived.Render(fmt.Sprintf("%s received \u2192 %s", fi.Filename, path))
	default:
		return fi.Filename
	}
}

func formatFileSize(bytes int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)
	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-3]) + "..."
}
