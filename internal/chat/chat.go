package chat

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/NathanBhanji/tail-chat/internal/crypto"
	tcnet "github.com/NathanBhanji/tail-chat/internal/net"
	"github.com/NathanBhanji/tail-chat/internal/protocol"
)

// Message is a decrypted chat message ready for display.
type Message struct {
	ID        string    `json:"id"`
	Sender    string    `json:"sender"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	IsOwn     bool      `json:"isOwn"`
	GroupID   string    `json:"groupID"`
}

// Group represents a group chat.
type Group struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Members []string `json:"members"`
}

// Manager coordinates chat sessions, message encryption, and delivery.
type Manager struct {
	mu           sync.RWMutex
	server       *tcnet.Server
	keyPair      *crypto.KeyPair
	hostname     string
	messages     map[string][]Message // chatKey -> messages (hostname or group:id)
	groups       map[string]*Group    // groupID -> group
	onMessage    func(chatKey string, msg Message)
	onGroupInvite func(invite *protocol.GroupInvite, from string)
}

// NewManager creates a chat manager.
func NewManager(server *tcnet.Server, kp *crypto.KeyPair, hostname string) *Manager {
	m := &Manager{
		server:   server,
		keyPair:  kp,
		hostname: hostname,
		messages: make(map[string][]Message),
		groups:   make(map[string]*Group),
	}

	server.OnMessage(m.handleMessage)
	server.OnConnect(m.handleConnect)

	return m
}

// OnMessage sets a callback for new messages (1:1 or group).
func (m *Manager) OnMessage(fn func(chatKey string, msg Message)) {
	m.onMessage = fn
}

// OnGroupInvite sets a callback for group invitations.
func (m *Manager) OnGroupInvite(fn func(invite *protocol.GroupInvite, from string)) {
	m.onGroupInvite = fn
}

func (m *Manager) handleConnect(c *tcnet.Connection) {
	// Connection established, nothing extra needed for now
}

func (m *Manager) handleMessage(c *tcnet.Connection, env *protocol.Envelope) {
	switch env.Type {
	case protocol.TypeChat:
		m.handleChatMessage(c, env)
	case protocol.TypeAck:
		// Message acknowledged
	case protocol.TypePing:
		m.handlePing(c)
	case protocol.TypeGroupInvite:
		m.handleGroupInvite(c, env)
	case protocol.TypeGroupChat:
		m.handleGroupChat(c, env)
	case protocol.TypeGroupAccept:
		m.handleGroupAccept(c, env)
	}
}

func (m *Manager) handleChatMessage(c *tcnet.Connection, env *protocol.Envelope) {
	chatMsg, err := protocol.Unwrap[protocol.ChatMessage](env)
	if err != nil {
		return
	}

	// Decrypt
	plaintext, err := crypto.Decrypt(c.SharedSecret, chatMsg.Ciphertext)
	if err != nil {
		return
	}

	msg := Message{
		ID:        chatMsg.ID,
		Sender:    c.PeerHostname,
		Content:   string(plaintext),
		Timestamp: time.Unix(0, chatMsg.Timestamp),
		IsOwn:     false,
	}

	chatKey := c.PeerHostname
	m.mu.Lock()
	m.messages[chatKey] = append(m.messages[chatKey], msg)
	m.mu.Unlock()

	// Send ack
	ack := &protocol.Ack{MessageID: chatMsg.ID}
	ackEnv, _ := protocol.Wrap(protocol.TypeAck, ack)
	protocol.WriteMessage(c.Conn, ackEnv)

	if m.onMessage != nil {
		m.onMessage(chatKey, msg)
	}
}

func (m *Manager) handlePing(c *tcnet.Connection) {
	pong := &protocol.Pong{Timestamp: time.Now().UnixNano()}
	env, _ := protocol.Wrap(protocol.TypePong, pong)
	protocol.WriteMessage(c.Conn, env)
}

func (m *Manager) handleGroupInvite(c *tcnet.Connection, env *protocol.Envelope) {
	invite, err := protocol.Unwrap[protocol.GroupInvite](env)
	if err != nil {
		return
	}

	if m.onGroupInvite != nil {
		m.onGroupInvite(invite, c.PeerHostname)
	}
}

func (m *Manager) handleGroupAccept(c *tcnet.Connection, env *protocol.Envelope) {
	accept, err := protocol.Unwrap[protocol.GroupAccept](env)
	if err != nil {
		return
	}

	m.mu.RLock()
	_, exists := m.groups[accept.GroupID]
	m.mu.RUnlock()

	if !exists {
		return
	}

	// Peer accepted — nothing else to do for now, they're already connected
}

func (m *Manager) handleGroupChat(c *tcnet.Connection, env *protocol.Envelope) {
	groupMsg, err := protocol.Unwrap[protocol.GroupChat](env)
	if err != nil {
		return
	}

	plaintext, err := crypto.Decrypt(c.SharedSecret, groupMsg.Ciphertext)
	if err != nil {
		return
	}

	msg := Message{
		ID:        groupMsg.ID,
		Sender:    groupMsg.Sender,
		Content:   string(plaintext),
		Timestamp: time.Unix(0, groupMsg.Timestamp),
		IsOwn:     false,
		GroupID:   groupMsg.GroupID,
	}

	chatKey := "group:" + groupMsg.GroupID
	m.mu.Lock()
	m.messages[chatKey] = append(m.messages[chatKey], msg)
	m.mu.Unlock()

	if m.onMessage != nil {
		m.onMessage(chatKey, msg)
	}
}

// SendMessage sends an encrypted 1:1 message to a connected peer.
func (m *Manager) SendMessage(peerHostname, content string) error {
	conn := m.server.GetConnection(peerHostname)
	if conn == nil {
		return fmt.Errorf("not connected to %s", peerHostname)
	}

	ciphertext, err := crypto.Encrypt(conn.SharedSecret, []byte(content))
	if err != nil {
		return fmt.Errorf("encrypt: %w", err)
	}

	now := time.Now()
	chatMsg := &protocol.ChatMessage{
		ID:         uuid.New().String(),
		Ciphertext: ciphertext,
		Timestamp:  now.UnixNano(),
	}

	if err := tcnet.SendEncrypted(conn, protocol.TypeChat, chatMsg); err != nil {
		return fmt.Errorf("send: %w", err)
	}

	// Store own message
	msg := Message{
		ID:        chatMsg.ID,
		Sender:    m.hostname,
		Content:   content,
		Timestamp: now,
		IsOwn:     true,
	}

	m.mu.Lock()
	m.messages[peerHostname] = append(m.messages[peerHostname], msg)
	m.mu.Unlock()

	if m.onMessage != nil {
		m.onMessage(peerHostname, msg)
	}

	return nil
}

// ConnectToPeer establishes a connection and registers it.
func (m *Manager) ConnectToPeer(ip string) (*tcnet.Connection, error) {
	addr := fmt.Sprintf("%s:%d", ip, tcnet.DefaultPort)
	conn, err := tcnet.Connect(addr, m.keyPair, m.hostname)
	if err != nil {
		return nil, err
	}
	m.server.AddConnection(conn)
	return conn, nil
}

// GetMessages returns chat history for a chat key.
func (m *Manager) GetMessages(chatKey string) []Message {
	m.mu.RLock()
	defer m.mu.RUnlock()
	msgs := m.messages[chatKey]
	result := make([]Message, len(msgs))
	copy(result, msgs)
	return result
}

// CreateGroup creates a new group chat and invites members.
func (m *Manager) CreateGroup(name string, memberHostnames []string) (*Group, error) {
	group := &Group{
		ID:      uuid.New().String(),
		Name:    name,
		Members: append(memberHostnames, m.hostname),
	}

	m.mu.Lock()
	m.groups[group.ID] = group
	m.mu.Unlock()

	// Send invites to all members
	invite := &protocol.GroupInvite{
		GroupID:   group.ID,
		GroupName: name,
		Members:   group.Members,
	}

	for _, host := range memberHostnames {
		conn := m.server.GetConnection(host)
		if conn == nil {
			continue
		}
		tcnet.SendEncrypted(conn, protocol.TypeGroupInvite, invite)
	}

	return group, nil
}

// AcceptGroupInvite accepts a group invitation.
func (m *Manager) AcceptGroupInvite(invite *protocol.GroupInvite, fromHost string) {
	group := &Group{
		ID:      invite.GroupID,
		Name:    invite.GroupName,
		Members: invite.Members,
	}

	m.mu.Lock()
	m.groups[group.ID] = group
	m.mu.Unlock()

	// Send accept back
	conn := m.server.GetConnection(fromHost)
	if conn != nil {
		accept := &protocol.GroupAccept{GroupID: group.ID}
		tcnet.SendEncrypted(conn, protocol.TypeGroupAccept, accept)
	}
}

// SendGroupMessage sends an encrypted message to all group members.
func (m *Manager) SendGroupMessage(groupID, content string) error {
	m.mu.RLock()
	group, exists := m.groups[groupID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("group %s not found", groupID)
	}

	now := time.Now()
	msgID := uuid.New().String()

	for _, member := range group.Members {
		if member == m.hostname {
			continue
		}

		conn := m.server.GetConnection(member)
		if conn == nil {
			continue
		}

		ciphertext, err := crypto.Encrypt(conn.SharedSecret, []byte(content))
		if err != nil {
			continue
		}

		groupMsg := &protocol.GroupChat{
			ID:         msgID,
			GroupID:    groupID,
			Sender:     m.hostname,
			Ciphertext: ciphertext,
			Timestamp:  now.UnixNano(),
		}

		tcnet.SendEncrypted(conn, protocol.TypeGroupChat, groupMsg)
	}

	// Store own message
	msg := Message{
		ID:        msgID,
		Sender:    m.hostname,
		Content:   content,
		Timestamp: now,
		IsOwn:     true,
		GroupID:   groupID,
	}

	chatKey := "group:" + groupID
	m.mu.Lock()
	m.messages[chatKey] = append(m.messages[chatKey], msg)
	m.mu.Unlock()

	if m.onMessage != nil {
		m.onMessage(chatKey, msg)
	}

	return nil
}

// Groups returns all groups.
func (m *Manager) Groups() []*Group {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var groups []*Group
	for _, g := range m.groups {
		groups = append(groups, g)
	}
	return groups
}

// IsConnected returns whether we have an active connection to a peer.
func (m *Manager) IsConnected(hostname string) bool {
	return m.server.GetConnection(hostname) != nil
}
