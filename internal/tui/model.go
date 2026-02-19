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

// Pane identifies which pane has focus.
type Pane int

const (
	PaneSidebar Pane = iota
	PaneChat
)

// View represents the right-pane content.
type View int

const (
	ViewChat View = iota
	ViewGroupCreate
	ViewSearch
	ViewEmpty // no chat selected yet
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

// FileSentMsg is sent after an async file send starts.
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

	// Layout
	focusPane Pane
	view      View // right-pane content

	// Sidebar state
	peers          []discovery.Peer
	peerCursor     int
	connecting     string
	connectingDots int

	// Chat state
	activeChatKey string
	messages      []chat.Message
	scrollOffset  int

	// Window
	width  int
	height int

	// Errors
	err       string
	errExpiry time.Time

	// Group creation
	groupName     string
	groupSelected map[string]bool
	groupCursor   int

	// Typing indicators
	lastTypingSent time.Time

	// Emoji tab completion
	emojiCompletions []string
	emojiCompIdx     int
	emojiReplaceFrom int

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
		focusPane:   PaneSidebar,
		view:        ViewEmpty,
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
		sw := m.sidebarWidth()
		m.input.Width = m.width - sw - 8
		return m, nil

	case IncomingMsg:
		if msg.ChatKey == m.activeChatKey {
			m.messages = m.chatMgr.GetMessages(m.activeChatKey)
			return m, nil
		}
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
		if msg.ChatKey == m.activeChatKey {
			m.messages = m.chatMgr.GetMessages(m.activeChatKey)
		}
		return m, nil

	case FileSentMsg:
		if msg.Err != nil {
			m.err = msg.Err.Error()
			m.errExpiry = time.Now().Add(3 * time.Second)
		}
		if msg.ChatKey == m.activeChatKey {
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
		m.chatMgr.AcceptGroupInvite(msg.Invite, msg.From)
		return m, nil

	case TypingMsg:
		return m, nil

	case ReactionUpdatedMsg:
		if msg.ChatKey == m.activeChatKey {
			m.messages = m.chatMgr.GetMessages(m.activeChatKey)
		}
		return m, nil

	case StatusUpdatedMsg:
		return m, nil

	case ErrorMsg:
		m.err = msg.Err.Error()
		m.errExpiry = time.Now().Add(5 * time.Second)
		return m, nil

	case TickMsg:
		if m.err != "" && time.Now().After(m.errExpiry) {
			m.err = ""
		}
		if m.connecting != "" {
			m.connectingDots = (m.connectingDots + 1) % 4
		}
		m.peers = m.peerWatcher.Peers()
		if m.activeChatKey != "" {
			m.messages = m.chatMgr.GetMessages(m.activeChatKey)
		}
		return m, tickCmd()
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// sidebarWidth returns the width of the sidebar pane.
func (m Model) sidebarWidth() int {
	sw := m.width / 4
	if sw < 20 {
		sw = 20
	}
	if sw > 35 {
		sw = 35
	}
	return sw
}

// openChat sets up the model to display a chat.
func (m Model) openChat(chatKey string) Model {
	m.activeChatKey = chatKey
	m.chatMgr.LoadMessages(chatKey)
	m.messages = m.chatMgr.GetMessages(chatKey)
	m.chatMgr.ClearUnread(chatKey)
	m.chatMgr.SendReadReceipts(chatKey)
	m.view = ViewChat
	m.focusPane = PaneChat
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

	if key == "ctrl+c" {
		return m, tea.Quit
	}

	// Tab switches focus between panes (unless emoji completing)
	if key == "tab" && m.focusPane == PaneChat && len(m.emojiCompletions) > 0 {
		return m.handleEmojiCompletion()
	}
	if key == "tab" && m.view != ViewGroupCreate && m.view != ViewSearch {
		if m.focusPane == PaneSidebar && m.activeChatKey != "" {
			m.focusPane = PaneChat
			m.input.Focus()
		} else {
			m.focusPane = PaneSidebar
			m.input.Blur()
		}
		return m, nil
	}

	if m.focusPane == PaneSidebar {
		return m.handleSidebarKeys(key)
	}

	// Right pane
	switch m.view {
	case ViewChat:
		return m.handleChatKeys(msg)
	case ViewGroupCreate:
		return m.handleGroupCreateKeys(msg)
	case ViewSearch:
		return m.handleSearchKeys(msg)
	}

	return m, nil
}

func (m Model) handleSidebarKeys(key string) (tea.Model, tea.Cmd) {
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
		if m.lastKey == "g" && time.Since(m.lastKeyTime) < 500*time.Millisecond {
			m.peerCursor = 0
			m.lastKey = ""
			return m, nil
		}
		m.lastKey = "g"
		m.lastKeyTime = time.Now()
		return m, nil

	case "G":
		if totalItems > 0 {
			m.peerCursor = totalItems - 1
		}

	case "/":
		m.view = ViewSearch
		m.focusPane = PaneChat
		m.searchResults = nil
		m.searchCursor = 0
		m.searchDone = false
		m.input.SetValue("")
		m.input.Placeholder = "Search messages..."
		m.input.Focus()
		return m, nil

	case "s":
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
		m.view = ViewGroupCreate
		m.focusPane = PaneChat
		m.groupName = ""
		m.groupSelected = make(map[string]bool)
		m.groupCursor = 0
		m.input.SetValue("")
		m.input.Placeholder = "Group name..."
		m.input.Focus()
		return m, nil

	case "r":
		m.peers = m.peerWatcher.Peers()
	}

	if key != "g" {
		m.lastKey = key
		m.lastKeyTime = time.Now()
	}

	return m, nil
}

func (m Model) selectPeerItem() (tea.Model, tea.Cmd) {
	groups := m.chatMgr.Groups()

	if m.peerCursor < len(m.peers) {
		peer := m.peers[m.peerCursor]
		if !peer.Online {
			m.err = fmt.Sprintf("%s is offline", peer.Hostname)
			m.errExpiry = time.Now().Add(3 * time.Second)
			return m, nil
		}

		if m.chatMgr.IsConnected(peer.Hostname) {
			m = m.openChat(peer.Hostname)
			return m, nil
		}

		if m.connecting != "" {
			return m, nil
		}

		ip := peer.TailscaleIP
		hostname := peer.Hostname
		m.connecting = hostname
		m.connectingDots = 0
		return m, func() tea.Msg {
			_, err := m.chatMgr.ConnectToPeer(ip)
			return ConnectedMsg{Hostname: hostname, Err: err}
		}
	}

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
		m.focusPane = PaneSidebar
		m.input.Blur()
		m.chatMgr.SendTyping(m.activeChatKey, false)
		return m, nil

	case "ctrl+u":
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

		// Handle /file command
		if strings.HasPrefix(content, "/file ") {
			path := strings.TrimSpace(content[6:])
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

		// Normal message
		content = expandEmoji(content)
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

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)

	if m.input.Value() != "" && time.Since(m.lastTypingSent) > 2*time.Second {
		m.lastTypingSent = time.Now()
		m.chatMgr.SendTyping(m.activeChatKey, true)
	}

	return m, cmd
}

func (m Model) handleEmojiCompletion() (tea.Model, tea.Cmd) {
	val := m.input.Value()

	if len(m.emojiCompletions) > 0 {
		m.emojiCompIdx = (m.emojiCompIdx + 1) % len(m.emojiCompletions)
		code := m.emojiCompletions[m.emojiCompIdx]
		newVal := val[:m.emojiReplaceFrom] + ":" + code + ":"
		m.input.SetValue(newVal)
		m.input.CursorEnd()
		return m, nil
	}

	lastColon := strings.LastIndexByte(val, ':')
	if lastColon < 0 || lastColon >= len(val)-1 {
		return m, nil
	}
	prefix := val[lastColon+1:]
	if strings.ContainsAny(prefix, " \t") || len(prefix) == 0 {
		return m, nil
	}

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

	if m.groupName == "" {
		switch key {
		case "esc":
			m.view = ViewChat
			m.focusPane = PaneSidebar
			m.input.Blur()
			if m.activeChatKey == "" {
				m.view = ViewEmpty
			}
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

	connectedPeers := m.connectedPeers()

	switch key {
	case "esc":
		m.focusPane = PaneSidebar
		if m.activeChatKey != "" {
			m.view = ViewChat
		} else {
			m.view = ViewEmpty
		}
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
		if m.groupCursor < len(connectedPeers) {
			host := connectedPeers[m.groupCursor]
			m.groupSelected[host] = !m.groupSelected[host]
		}

	case "ctrl+s":
		var members []string
		for host, sel := range m.groupSelected {
			if sel {
				members = append(members, host)
			}
		}
		if len(members) > 0 {
			m.chatMgr.CreateGroup(m.groupName, members)
			m.focusPane = PaneSidebar
			if m.activeChatKey != "" {
				m.view = ViewChat
			} else {
				m.view = ViewEmpty
			}
		}
		return m, nil
	}

	return m, nil
}

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
		m.focusPane = PaneSidebar
		m.input.Blur()
		m.searchResults = nil
		if m.activeChatKey != "" {
			m.view = ViewChat
		} else {
			m.view = ViewEmpty
		}
		return m, nil

	case "enter":
		if !m.searchDone {
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
			sort.Slice(m.searchResults, func(i, j int) bool {
				return m.searchResults[i].Message.Timestamp.After(m.searchResults[j].Message.Timestamp)
			})
			m.searchDone = true
			m.searchCursor = 0
			m.input.Blur()
			return m, nil
		}

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
		if m.searchDone {
			m.searchDone = false
			m.searchResults = nil
			m.input.SetValue("")
			m.input.Focus()
			return m, nil
		}
	}

	if !m.searchDone {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	return m, nil
}

// --- View ---

func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	sw := m.sidebarWidth()
	chatWidth := m.width - sw - 3 // borders + padding

	sidebar := m.renderSidebar(sw)
	var rightPane string

	switch m.view {
	case ViewChat:
		rightPane = m.renderChat(chatWidth)
	case ViewGroupCreate:
		rightPane = m.renderGroupCreate(chatWidth)
	case ViewSearch:
		rightPane = m.renderSearch(chatWidth)
	default:
		rightPane = m.renderEmpty(chatWidth)
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, sidebar, rightPane)
}

func (m Model) renderSidebar(width int) string {
	h := m.height - 2 // border
	innerW := width - 4 // border + padding

	var b strings.Builder

	// Title
	b.WriteString(sidebarTitle.Render("tailchat"))
	b.WriteString("\n")

	// Self info
	selfHost := m.peerWatcher.SelfHostname()
	selfStatus := m.chatMgr.GetStatus(selfHost)
	statusLabel := ""
	if selfStatus != "" && selfStatus != "available" {
		statusLabel = " " + statusText(selfStatus)
	}
	self := fmt.Sprintf("%s %s%s",
		peerOnline.Render("\u25cf"),
		truncate(selfHost, innerW-4),
		statusLabel)
	b.WriteString(self)
	b.WriteString("\n\n")

	linesUsed := 4

	// Peers
	onlineCount := 0
	for _, p := range m.peers {
		if p.Online {
			onlineCount++
		}
	}
	b.WriteString(helpStyle.Render(fmt.Sprintf("Peers (%d)", onlineCount)))
	b.WriteString("\n")
	linesUsed++

	idx := 0
	for _, peer := range m.peers {
		if linesUsed >= h-4 {
			break
		}

		var dot string
		switch {
		case m.chatMgr.IsConnected(peer.Hostname):
			dot = encryptedBadge.Render("\u25cf")
		case peer.RunningTailchat:
			dot = tailchatOnline.Render("\u25cf")
		case peer.Online:
			dot = peerOnline.Render("\u25cb")
		default:
			dot = peerOffline.Render("\u25cb")
		}

		name := truncate(peer.Hostname, innerW-6)
		unreadBadge := ""
		if n := m.chatMgr.Unread(peer.Hostname); n > 0 {
			unreadBadge = unreadStyle.Render(fmt.Sprintf(" %d", n))
		}

		line := fmt.Sprintf("%s %s%s", dot, name, unreadBadge)

		if idx == m.peerCursor && m.focusPane == PaneSidebar {
			b.WriteString(peerSelected.Render(line))
		} else if peer.Hostname == m.activeChatKey {
			b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(text).Render(line))
		} else {
			b.WriteString(peerNormal.Render(line))
		}
		b.WriteString("\n")
		idx++
		linesUsed++
	}

	// Groups
	groups := m.chatMgr.Groups()
	if len(groups) > 0 && linesUsed < h-4 {
		b.WriteString("\n")
		b.WriteString(helpStyle.Render("Groups"))
		b.WriteString("\n")
		linesUsed += 2

		for _, g := range groups {
			if linesUsed >= h-4 {
				break
			}
			chatKey := "group:" + g.ID
			name := fmt.Sprintf("%s %s", groupBadge.Render("#"), truncate(g.Name, innerW-6))
			unreadBadge := ""
			if n := m.chatMgr.Unread(chatKey); n > 0 {
				unreadBadge = unreadStyle.Render(fmt.Sprintf(" %d", n))
			}
			line := name + unreadBadge

			if idx == m.peerCursor && m.focusPane == PaneSidebar {
				b.WriteString(peerSelected.Render(line))
			} else if chatKey == m.activeChatKey {
				b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(text).Render(line))
			} else {
				b.WriteString(peerNormal.Render(line))
			}
			b.WriteString("\n")
			idx++
			linesUsed++
		}
	}

	// Connecting indicator
	if m.connecting != "" && linesUsed < h-2 {
		dots := strings.Repeat(".", m.connectingDots+1)
		b.WriteString("\n")
		b.WriteString(connectingStyle.Render(truncate(fmt.Sprintf("%s%s", m.connecting, dots), innerW)))
		b.WriteString("\n")
		linesUsed += 2
	}

	// Pad remaining height
	for linesUsed < h-2 {
		b.WriteString("\n")
		linesUsed++
	}

	// Help at bottom
	if m.focusPane == PaneSidebar {
		b.WriteString(helpStyle.Render("j/k \u2022 enter \u2022 tab \u2192"))
	} else {
		b.WriteString(helpStyle.Render("tab \u2190 focus"))
	}

	style := sidebarBlurred.Width(width - 2).Height(h)
	if m.focusPane == PaneSidebar {
		style = sidebarFocused.Width(width - 2).Height(h)
	}
	return style.Render(b.String())
}

func (m Model) renderChat(width int) string {
	h := m.height - 2
	innerW := width - 4

	var b strings.Builder

	// Header
	chatName := m.activeChatKey
	if strings.HasPrefix(chatName, "group:") {
		for _, g := range m.chatMgr.Groups() {
			if "group:"+g.ID == m.activeChatKey {
				chatName = "# " + g.Name
				break
			}
		}
	}

	peerStatus := ""
	if !strings.HasPrefix(m.activeChatKey, "group:") {
		state := m.chatMgr.GetStatus(m.activeChatKey)
		if state != "" && state != "available" {
			peerStatus = " " + statusText(state)
		}
	}

	header := fmt.Sprintf("%s%s %s", chatName, peerStatus, encryptedBadge.Render("\U0001f512"))
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(primary).Render(truncate(header, innerW)))
	b.WriteString("\n\n")

	// Calculate space
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

	availHeight := h - 7 - extraLines
	if availHeight < 3 {
		availHeight = 3
	}

	// Message window with scrollback
	msgs := m.messages
	endIdx := len(msgs) - m.scrollOffset
	if endIdx > len(msgs) {
		endIdx = len(msgs)
	}
	if endIdx < 0 {
		endIdx = 0
	}

	linesUsed := 0
	startIdx := endIdx
	for i := endIdx - 1; i >= 0; i-- {
		lines := 1
		if len(msgs[i].Reactions) > 0 {
			lines++
		}
		if linesUsed+lines > availHeight {
			break
		}
		linesUsed += lines
		startIdx = i
	}

	for i := startIdx; i < endIdx; i++ {
		msg := msgs[i]
		ts := msgTimeStyle.Render(msg.Timestamp.Format("15:04"))

		if msg.Sender == "system" {
			b.WriteString(fmt.Sprintf(" %s %s\n", ts, systemMsgStyle.Render(msg.Content)))
			continue
		}

		if msg.FileInfo != nil {
			var sender string
			if msg.IsOwn {
				sender = ownMsgStyle.Render("you")
			} else {
				sender = peerMsgStyle.Render(msg.Sender)
			}
			b.WriteString(fmt.Sprintf(" %s %s: %s\n", ts, sender, renderFileTransfer(msg.FileInfo)))
			continue
		}

		var sender string
		if msg.IsOwn {
			sender = ownMsgStyle.Render("you")
		} else {
			sender = peerMsgStyle.Render(msg.Sender)
		}

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

		rendered := renderContent(msg.Content)
		b.WriteString(fmt.Sprintf(" %s %s: %s%s\n", ts, sender, rendered, tick))

		if len(msg.Reactions) > 0 {
			b.WriteString(fmt.Sprintf(" %s %s\n", strings.Repeat(" ", 5), renderReactions(msg.Reactions)))
		}
	}

	if len(msgs) == 0 {
		b.WriteString(helpStyle.Render(" No messages yet. Say hello!\n"))
		linesUsed = 1
	}

	for i := linesUsed; i < availHeight; i++ {
		b.WriteString("\n")
	}

	if m.scrollOffset > 0 {
		b.WriteString(scrollIndicator.Render(fmt.Sprintf(" \u2191 %d older messages", m.scrollOffset)))
		b.WriteString("\n")
	}

	if isTyping {
		b.WriteString(typingStyle.Render(fmt.Sprintf(" %s is typing...", m.activeChatKey)))
		b.WriteString("\n")
	}

	if m.err != "" {
		b.WriteString(errorStyle.Render(" " + m.err))
		b.WriteString("\n")
	}

	if len(m.emojiCompletions) > 0 {
		maxShow := 6
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
			parts = append(parts, helpStyle.Render(fmt.Sprintf("+%d", len(m.emojiCompletions)-maxShow)))
		}
		b.WriteString(" " + strings.Join(parts, ""))
		b.WriteString("\n")
	}

	// Input
	m.input.Width = innerW - 2
	b.WriteString(inputStyle.Width(innerW).Render(m.input.View()))
	b.WriteString("\n")

	// Help
	b.WriteString(helpStyle.Render(" enter send \u2022 /react /file \u2022 esc \u2190"))

	style := chatBlurred.Width(width - 2).Height(h)
	if m.focusPane == PaneChat {
		style = chatFocused.Width(width - 2).Height(h)
	}
	return style.Render(b.String())
}

func (m Model) renderEmpty(width int) string {
	h := m.height - 2
	innerW := width - 4

	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(primary).Render("tailchat"))
	b.WriteString("\n\n")

	welcome := []string{
		"Select a peer to start chatting.",
		"",
		fmt.Sprintf("%s connected  %s tailchat",
			encryptedBadge.Render("\u25cf"), tailchatOnline.Render("\u25cf")),
		fmt.Sprintf("%s online     %s offline",
			peerOnline.Render("\u25cb"), peerOffline.Render("\u25cb")),
		"",
		"j/k navigate \u2022 enter connect",
		"/ search \u2022 n new group",
		"s cycle status \u2022 q quit",
	}

	for _, line := range welcome {
		b.WriteString(helpStyle.Render(truncate(line, innerW)))
		b.WriteString("\n")
	}

	// Pad
	for i := len(welcome) + 3; i < h-2; i++ {
		b.WriteString("\n")
	}

	style := chatBlurred.Width(width - 2).Height(h)
	return style.Render(b.String())
}

func (m Model) renderGroupCreate(width int) string {
	h := m.height - 2
	innerW := width - 4

	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(primary).Render("New Group"))
	b.WriteString("\n\n")

	if m.groupName == "" {
		b.WriteString("Enter group name:\n\n")
		m.input.Width = innerW - 2
		b.WriteString(inputStyle.Width(innerW).Render(m.input.View()))
		b.WriteString("\n\n")
		b.WriteString(helpStyle.Render("enter confirm \u2022 esc cancel"))
	} else {
		b.WriteString(fmt.Sprintf("Group: %s\n\n", groupBadge.Render("# "+m.groupName)))
		b.WriteString(helpStyle.Render("Select members"))
		b.WriteString("\n")

		connectedPeers := m.connectedPeers()
		if len(connectedPeers) == 0 {
			b.WriteString(helpStyle.Render("No connected peers.\n"))
		}

		for i, host := range connectedPeers {
			check := peerOffline.Render("[ ]")
			if m.groupSelected[host] {
				check = encryptedBadge.Render("[x]")
			}

			line := fmt.Sprintf("%s %s", check, truncate(host, innerW-6))
			if i == m.groupCursor {
				b.WriteString(peerSelected.Render(line))
			} else {
				b.WriteString(peerNormal.Render(line))
			}
			b.WriteString("\n")
		}

		selected := 0
		for _, sel := range m.groupSelected {
			if sel {
				selected++
			}
		}
		if selected > 0 {
			b.WriteString(fmt.Sprintf("\n%s\n", encryptedBadge.Render(fmt.Sprintf("%d selected", selected))))
		}

		b.WriteString("\n")
		b.WriteString(helpStyle.Render("j/k \u2022 space toggle \u2022 ctrl+s create \u2022 esc cancel"))
	}

	// Pad
	content := b.String()
	lines := strings.Count(content, "\n")
	for i := lines; i < h-2; i++ {
		content += "\n"
	}

	style := chatFocused.Width(width - 2).Height(h)
	return style.Render(content)
}

func (m Model) renderSearch(width int) string {
	h := m.height - 2
	innerW := width - 4

	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(primary).Render("Search"))
	b.WriteString("\n\n")

	if !m.searchDone {
		b.WriteString("Enter search query:\n\n")
		m.input.Width = innerW - 2
		b.WriteString(inputStyle.Width(innerW).Render(m.input.View()))
		b.WriteString("\n\n")
		b.WriteString(helpStyle.Render("enter search \u2022 esc cancel"))
	} else {
		b.WriteString(fmt.Sprintf("Results: %d matches\n\n", len(m.searchResults)))

		maxShow := h - 8
		if maxShow < 5 {
			maxShow = 5
		}
		if maxShow > len(m.searchResults) {
			maxShow = len(m.searchResults)
		}

		for i := 0; i < maxShow; i++ {
			r := m.searchResults[i]
			ts := r.Message.Timestamp.Format("Jan 02 15:04")
			line := fmt.Sprintf("[%s] %s: %s",
				helpStyle.Render(ts),
				peerMsgStyle.Render(r.Message.Sender),
				truncate(r.Message.Content, innerW-30),
			)
			if i == m.searchCursor {
				b.WriteString(peerSelected.Render(line))
			} else {
				b.WriteString(peerNormal.Render(line))
			}
			b.WriteString("\n")
		}

		if len(m.searchResults) == 0 {
			b.WriteString(helpStyle.Render("No results found.\n"))
		}

		b.WriteString("\n")
		b.WriteString(helpStyle.Render("j/k \u2022 enter open \u2022 / new search \u2022 esc back"))
	}

	content := b.String()
	lines := strings.Count(content, "\n")
	for i := lines; i < h-2; i++ {
		content += "\n"
	}

	style := chatFocused.Width(width - 2).Height(h)
	return style.Render(content)
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
	if max < 4 {
		max = 4
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-3]) + "..."
}
