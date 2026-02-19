package tui

import (
	"fmt"
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

// ErrorMsg is a transient error to display.
type ErrorMsg struct {
	Err error
}

// TickMsg is sent periodically for UI updates.
type TickMsg time.Time

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
	groupName    string
	groupMembers []string
	groupCursor  int

	// Pending group invites
	pendingInvites []pendingInvite

	// Components
	input textinput.Model
}

type pendingInvite struct {
	invite *protocol.GroupInvite
	from   string
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
	var cmds []tea.Cmd

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
		}
		// Always return so the view re-renders (unread badges update)
		return m, nil

	case PeerConnectMsg:
		// A peer connected inbound — refresh so the lock icon shows up
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
			// Switch to chat view
			m.activeChatKey = msg.Hostname
			m.messages = m.chatMgr.GetMessages(m.activeChatKey)
			m.chatMgr.ClearUnread(m.activeChatKey)
			m.view = ViewChat
			m.input.Placeholder = "Type a message..."
			m.input.SetValue("")
			m.input.Focus()
		}
		return m, nil

	case GroupInviteMsg:
		m.pendingInvites = append(m.pendingInvites, pendingInvite{
			invite: msg.Invite,
			from:   msg.From,
		})
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
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Global keys
	switch key {
	case "ctrl+c", "ctrl+d":
		return m, tea.Quit
	}

	switch m.view {
	case ViewPeers:
		return m.handlePeerKeys(key)
	case ViewChat:
		return m.handleChatKeys(msg)
	case ViewGroupCreate:
		return m.handleGroupCreateKeys(msg)
	}

	return m, nil
}

func (m Model) handlePeerKeys(key string) (tea.Model, tea.Cmd) {
	totalItems := len(m.peers) + len(m.chatMgr.Groups()) + len(m.pendingInvites)

	switch key {
	case "q":
		return m, tea.Quit

	case "up", "k":
		if m.peerCursor > 0 {
			m.peerCursor--
		}

	case "down", "j":
		if m.peerCursor < totalItems-1 {
			m.peerCursor++
		}

	case "enter":
		return m.selectPeerItem()

	case "g":
		// Create group
		m.view = ViewGroupCreate
		m.groupName = ""
		m.groupMembers = nil
		m.groupCursor = 0
		m.input.SetValue("")
		m.input.Placeholder = "Group name..."
		m.input.Focus()
		return m, nil

	case "a":
		// Accept first pending invite
		if len(m.pendingInvites) > 0 {
			inv := m.pendingInvites[0]
			m.chatMgr.AcceptGroupInvite(inv.invite, inv.from)
			m.pendingInvites = m.pendingInvites[1:]
		}

	case "r":
		// Refresh peers
		m.peers = m.peerWatcher.Peers()
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
			m.activeChatKey = peer.Hostname
			m.messages = m.chatMgr.GetMessages(m.activeChatKey)
			m.chatMgr.ClearUnread(m.activeChatKey)
			m.view = ViewChat
			m.input.Placeholder = "Type a message..."
			m.input.SetValue("")
			m.input.Focus()
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
		m.activeChatKey = "group:" + group.ID
		m.messages = m.chatMgr.GetMessages(m.activeChatKey)
		m.chatMgr.ClearUnread(m.activeChatKey)
		m.view = ViewChat
		m.input.Placeholder = "Type a message..."
		m.input.SetValue("")
		m.input.Focus()
		return m, nil
	}

	// Check if cursor is on a pending invite
	inviteIdx := m.peerCursor - len(m.peers) - len(groups)
	if inviteIdx >= 0 && inviteIdx < len(m.pendingInvites) {
		inv := m.pendingInvites[inviteIdx]
		m.chatMgr.AcceptGroupInvite(inv.invite, inv.from)
		m.pendingInvites = append(m.pendingInvites[:inviteIdx], m.pendingInvites[inviteIdx+1:]...)
	}

	return m, nil
}

func (m Model) handleChatKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "esc":
		m.view = ViewPeers
		m.input.Blur()
		return m, nil

	case "enter":
		content := strings.TrimSpace(m.input.Value())
		if content == "" {
			return m, nil
		}
		m.input.SetValue("")

		chatKey := m.activeChatKey
		if strings.HasPrefix(chatKey, "group:") {
			groupID := strings.TrimPrefix(chatKey, "group:")
			if err := m.chatMgr.SendGroupMessage(groupID, content); err != nil {
				m.err = err.Error()
				m.errExpiry = time.Now().Add(3 * time.Second)
			}
		} else {
			if err := m.chatMgr.SendMessage(chatKey, content); err != nil {
				m.err = err.Error()
				m.errExpiry = time.Now().Add(3 * time.Second)
			}
		}

		m.messages = m.chatMgr.GetMessages(m.activeChatKey)
		return m, nil
	}

	// Pass to text input
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m Model) handleGroupCreateKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

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

		if m.groupName == "" {
			// First enter sets group name
			m.groupName = val
			m.input.SetValue("")
			m.input.Placeholder = "Add member hostname (enter to add, esc when done)..."
			return m, nil
		}

		// Add member
		m.groupMembers = append(m.groupMembers, val)
		m.input.SetValue("")
		return m, nil

	case "ctrl+s":
		// Save group
		if m.groupName != "" && len(m.groupMembers) > 0 {
			m.chatMgr.CreateGroup(m.groupName, m.groupMembers)
			m.view = ViewPeers
			m.input.Blur()
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
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
	}

	return ""
}

func (m Model) viewPeers() string {
	var b strings.Builder

	// Title
	title := titleStyle.Render(" tailchat ")
	b.WriteString(title)
	b.WriteString("\n\n")

	// Self info
	selfHost := m.peerWatcher.SelfHostname()
	selfIP := m.peerWatcher.SelfIP()
	b.WriteString(fmt.Sprintf("  %s  %s\n",
		peerOnline.Render("●"),
		lipgloss.NewStyle().Foreground(text).Bold(true).Render(
			fmt.Sprintf("%s (%s)", selfHost, selfIP)),
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
		dot := peerOffline.Render("○")
		if peer.Online {
			dot = peerOnline.Render("●")
		}

		connStatus := ""
		if m.chatMgr.IsConnected(peer.Hostname) {
			connStatus = encryptedBadge.Render(" 🔒")
		}

		unreadBadge := ""
		if n := m.chatMgr.Unread(peer.Hostname); n > 0 {
			unreadBadge = unreadStyle.Render(fmt.Sprintf(" (%d)", n))
		}

		name := fmt.Sprintf("%s %s  %s%s%s", dot, peer.Hostname, helpStyle.Render(peer.TailscaleIP), connStatus, unreadBadge)

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

	// Pending invites
	if len(m.pendingInvites) > 0 {
		b.WriteString("\n")
		b.WriteString(sidebarTitle.Render("  Invites"))
		b.WriteString("\n")

		for _, inv := range m.pendingInvites {
			name := fmt.Sprintf("  %s from %s (%d members)",
				connectingStyle.Render("⬤ "+inv.invite.GroupName),
				inv.from,
				len(inv.invite.Members),
			)
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
	b.WriteString(helpStyle.Render("  j/k navigate • enter connect/open • g new group • r refresh • q quit"))

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

	header := fmt.Sprintf(" tailchat — %s %s",
		chatName,
		encryptedBadge.Render("e2e encrypted"),
	)
	b.WriteString(titleStyle.Render(header))
	b.WriteString("\n\n")

	// Messages
	availHeight := m.height - 8
	if availHeight < 5 {
		availHeight = 5
	}

	msgs := m.messages
	// Show only last N messages that fit
	startIdx := 0
	if len(msgs) > availHeight {
		startIdx = len(msgs) - availHeight
	}

	for i := startIdx; i < len(msgs); i++ {
		msg := msgs[i]
		ts := msgTimeStyle.Render(msg.Timestamp.Format("15:04"))

		if msg.Sender == "system" {
			b.WriteString(fmt.Sprintf("  %s %s\n",
				ts, systemMsgStyle.Render(msg.Content)))
			continue
		}

		var sender string
		if msg.IsOwn {
			sender = ownMsgStyle.Render("you")
		} else {
			sender = peerMsgStyle.Render(msg.Sender)
		}

		b.WriteString(fmt.Sprintf("  %s %s: %s\n",
			ts, sender, msgContentStyle.Render(msg.Content)))
	}

	if len(msgs) == 0 {
		b.WriteString(helpStyle.Render("  No messages yet. Say hello!\n"))
	}

	// Pad to fill space
	msgLines := len(msgs) - startIdx
	if msgLines == 0 {
		msgLines = 1
	}
	for i := msgLines; i < availHeight; i++ {
		b.WriteString("\n")
	}

	// Error in chat view
	if m.err != "" {
		b.WriteString(errorStyle.Render("  " + m.err))
		b.WriteString("\n")
	}

	// Input
	b.WriteString("\n")
	b.WriteString(inputStyle.Render(m.input.View()))
	b.WriteString("\n")

	// Help
	b.WriteString(helpStyle.Render("  enter send • esc back"))

	return appStyle.Render(b.String())
}

func (m Model) viewGroupCreate() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render(" tailchat — New Group "))
	b.WriteString("\n\n")

	if m.groupName == "" {
		b.WriteString("  Enter group name:\n\n")
	} else {
		b.WriteString(fmt.Sprintf("  Group: %s\n", groupBadge.Render("# "+m.groupName)))
		b.WriteString("\n")

		if len(m.groupMembers) > 0 {
			b.WriteString("  Members:\n")
			for _, member := range m.groupMembers {
				b.WriteString(fmt.Sprintf("    %s %s\n", peerOnline.Render("●"), member))
			}
			b.WriteString("\n")
		}

		b.WriteString("  Add member hostname:\n\n")
	}

	b.WriteString(inputStyle.Render(m.input.View()))
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("  enter add • ctrl+s create group • esc cancel"))

	return appStyle.Render(b.String())
}
