package chat

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/NathanBhanji/tail-chat/internal/crypto"
	tcnet "github.com/NathanBhanji/tail-chat/internal/net"
	"github.com/NathanBhanji/tail-chat/internal/protocol"
	"github.com/NathanBhanji/tail-chat/internal/storage"
)

// DeliveryState tracks message delivery status.
type DeliveryState int

const (
	StateSending   DeliveryState = iota
	StateDelivered               // ACK received
	StateRead                    // Read receipt received
)

// FileState tracks file transfer progress.
type FileState int

const (
	FileSending FileState = iota
	FileSent
	FileFailed
	FileReceived
)

// FileInfo describes a file transfer attached to a message.
type FileInfo struct {
	Filename string
	Size     int64
	State    FileState
	Error    string // non-empty when FileFailed
	Path     string // destination path when FileReceived
}

// Reaction is a single emoji reaction on a message.
type Reaction struct {
	Emoji  string
	Sender string
}

// Message is a decrypted chat message ready for display.
type Message struct {
	ID        string
	Sender    string
	Content   string
	Timestamp time.Time
	IsOwn     bool
	GroupID   string
	State     DeliveryState
	Reactions []Reaction
	FileInfo  *FileInfo
}

// Group represents a group chat.
type Group struct {
	ID      string
	Name    string
	Members []string
}

// fileChunkSize is the raw bytes per FileData chunk (64KB).
const fileChunkSize = 64 * 1024

// maxMessagesPerChat caps in-memory message history per chat to prevent OOM.
const maxMessagesPerChat = 1000

// fileTransfer tracks an in-progress incoming file transfer.
type fileTransfer struct {
	chatKey  string
	msgID    string
	filename string
	size     int64
	checksum string // expected SHA-256 hex
	file     *os.File
	received int64
}

// persistRequest is a request to persist messages for a chat key.
type persistRequest struct {
	chatKey string
	msgs    []storage.StoredMessage
}

// Manager coordinates chat sessions, message encryption, and delivery.
type Manager struct {
	mu              sync.RWMutex
	server          *tcnet.Server
	keyPair         *crypto.KeyPair
	hostname        string
	messages        map[string][]Message
	groups          map[string]*Group
	unread          map[string]int
	typing          map[string]time.Time
	peerStatus      map[string]string
	store           *storage.Store
	activeTransfers map[string]*fileTransfer // transfer ID -> state
	onMessage       func(chatKey string, msg Message)
	onGroupInvite   func(invite *protocol.GroupInvite, from string)
	onPeerConnect   func(hostname string)
	onTyping        func(chatKey string, isTyping bool)
	onReaction      func(chatKey string, msgID string)
	onStatus        func(hostname string, state string)

	// TOFU key pinning
	knownKeys *crypto.KnownKeys

	// Serialized persistence
	persistCh chan persistRequest

	// Auto-reconnect
	reconnectTargets map[string]string
	reconnectStop    chan struct{}
	stopOnce         sync.Once
}

// NewManager creates a chat manager.
func NewManager(server *tcnet.Server, kp *crypto.KeyPair, hostname string, store *storage.Store, knownKeys *crypto.KnownKeys) *Manager {
	m := &Manager{
		server:           server,
		keyPair:          kp,
		hostname:         hostname,
		messages:         make(map[string][]Message),
		groups:           make(map[string]*Group),
		unread:           make(map[string]int),
		typing:           make(map[string]time.Time),
		peerStatus:       make(map[string]string),
		store:            store,
		knownKeys:        knownKeys,
		activeTransfers:  make(map[string]*fileTransfer),
		reconnectTargets: make(map[string]string),
		reconnectStop:    make(chan struct{}),
		persistCh:        make(chan persistRequest, 64),
	}

	server.OnMessage(m.handleMessage)
	server.OnConnect(m.handleConnect)
	server.OnDisconnect(m.handleDisconnect)

	if store != nil {
		m.loadPersistedData()
	}

	go m.reconnectLoop()
	go m.persistWorker()

	return m
}

func (m *Manager) loadPersistedData() {
	groups, err := m.store.LoadGroups()
	if err != nil {
		return
	}
	m.mu.Lock()
	for _, sg := range groups {
		m.groups[sg.ID] = &Group{ID: sg.ID, Name: sg.Name, Members: sg.Members}
	}
	m.mu.Unlock()
}

// LoadMessages loads persisted messages for a chat key.
func (m *Manager) LoadMessages(chatKey string) {
	if m.store == nil {
		return
	}
	stored, err := m.store.LoadMessages(chatKey)
	if err != nil || len(stored) == 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.messages[chatKey]) > 0 {
		return
	}
	for _, sm := range stored {
		var reactions []Reaction
		for _, sr := range sm.Reactions {
			reactions = append(reactions, Reaction{Emoji: sr.Emoji, Sender: sr.Sender})
		}
		state := StateSending
		if sm.Read {
			state = StateRead
		} else if sm.Delivered {
			state = StateDelivered
		}
		msg := Message{
			ID:        sm.ID,
			Sender:    sm.Sender,
			Content:   sm.Content,
			Timestamp: sm.Timestamp,
			IsOwn:     sm.IsOwn,
			GroupID:   sm.GroupID,
			State:     state,
			Reactions: reactions,
		}
		if sm.FileInfo != nil {
			msg.FileInfo = &FileInfo{
				Filename: sm.FileInfo.Filename,
				Size:     sm.FileInfo.Size,
				State:    FileState(sm.FileInfo.State),
				Error:    sm.FileInfo.Error,
				Path:     sm.FileInfo.Path,
			}
		}
		m.messages[chatKey] = append(m.messages[chatKey], msg)
	}
}

func (m *Manager) persistMessages(chatKey string) {
	if m.store == nil {
		return
	}
	msgs := m.messages[chatKey]
	var stored []storage.StoredMessage
	for _, msg := range msgs {
		var reactions []storage.StoredReaction
		for _, r := range msg.Reactions {
			reactions = append(reactions, storage.StoredReaction{Emoji: r.Emoji, Sender: r.Sender})
		}
		sm := storage.StoredMessage{
			ID:        msg.ID,
			Sender:    msg.Sender,
			Content:   msg.Content,
			Timestamp: msg.Timestamp,
			IsOwn:     msg.IsOwn,
			GroupID:   msg.GroupID,
			Delivered: msg.State >= StateDelivered,
			Read:      msg.State >= StateRead,
			Reactions: reactions,
		}
		if msg.FileInfo != nil {
			sm.FileInfo = &storage.StoredFileInfo{
				Filename: msg.FileInfo.Filename,
				Size:     msg.FileInfo.Size,
				State:    int(msg.FileInfo.State),
				Error:    msg.FileInfo.Error,
				Path:     msg.FileInfo.Path,
			}
		}
		stored = append(stored, sm)
	}
	select {
	case m.persistCh <- persistRequest{chatKey: chatKey, msgs: stored}:
	default:
		// Channel full, persist synchronously to avoid data loss
		m.store.SaveMessages(chatKey, stored)
	}
}

func (m *Manager) persistWorker() {
	for req := range m.persistCh {
		m.store.SaveMessages(req.chatKey, req.msgs)
	}
}

func (m *Manager) persistGroups() {
	if m.store == nil {
		return
	}
	var groups []storage.StoredGroup
	for _, g := range m.groups {
		groups = append(groups, storage.StoredGroup{ID: g.ID, Name: g.Name, Members: g.Members})
	}
	go m.store.SaveGroups(groups)
}

// trimMessages ensures a chat's in-memory history doesn't exceed maxMessagesPerChat.
// Must be called with m.mu held.
func (m *Manager) trimMessages(chatKey string) {
	msgs := m.messages[chatKey]
	if len(msgs) > maxMessagesPerChat {
		m.messages[chatKey] = msgs[len(msgs)-maxMessagesPerChat:]
	}
}

// Callbacks
func (m *Manager) OnMessage(fn func(chatKey string, msg Message)) { m.onMessage = fn }
func (m *Manager) OnGroupInvite(fn func(invite *protocol.GroupInvite, from string)) {
	m.onGroupInvite = fn
}
func (m *Manager) OnPeerConnect(fn func(hostname string))           { m.onPeerConnect = fn }
func (m *Manager) OnTyping(fn func(chatKey string, isTyping bool))  { m.onTyping = fn }
func (m *Manager) OnReaction(fn func(chatKey string, msgID string)) { m.onReaction = fn }
func (m *Manager) OnStatus(fn func(hostname string, state string))  { m.onStatus = fn }

func (m *Manager) handleConnect(c *tcnet.Connection) {
	if m.onPeerConnect != nil {
		m.onPeerConnect(c.PeerHostname)
	}
	m.mu.RLock()
	state := m.peerStatus[m.hostname]
	m.mu.RUnlock()
	if state == "" {
		state = "available"
	}
	env, err := protocol.Wrap(protocol.TypeStatus, &protocol.Status{State: state})
	if err != nil {
		return
	}
	c.WriteMessage(env)
}

func (m *Manager) handleDisconnect(hostname string) {
	m.mu.Lock()
	delete(m.peerStatus, hostname)
	m.mu.Unlock()
}

func (m *Manager) handleMessage(c *tcnet.Connection, env *protocol.Envelope) {
	switch env.Type {
	case protocol.TypeChat:
		m.handleChatMessage(c, env)
	case protocol.TypeAck:
		m.handleAck(c, env)
	case protocol.TypePing:
		m.handlePing(c)
	case protocol.TypeGroupInvite:
		m.handleGroupInvite(c, env)
	case protocol.TypeGroupChat:
		m.handleGroupChat(c, env)
	case protocol.TypeGroupAccept:
		m.handleGroupAccept(c, env)
	case protocol.TypeTyping:
		m.handleTyping(c, env)
	case protocol.TypeReaction:
		m.handleReaction(c, env)
	case protocol.TypeStatus:
		m.handleStatusMsg(c, env)
	case protocol.TypeReadReceipt:
		m.handleReadReceipt(c, env)
	case protocol.TypeFileOffer:
		m.handleFileOffer(c, env)
	case protocol.TypeFileData:
		m.handleFileData(c, env)
	case protocol.TypeFileComplete:
		m.handleFileComplete(c, env)
	}
}

func (m *Manager) handleChatMessage(c *tcnet.Connection, env *protocol.Envelope) {
	chatMsg, err := protocol.Unwrap[protocol.ChatMessage](env)
	if err != nil {
		m.systemMessage(c.PeerHostname, fmt.Sprintf("[failed to parse message: %v]", err))
		return
	}
	plaintext, err := crypto.Decrypt(c.SharedSecret, chatMsg.Ciphertext)
	if err != nil {
		m.systemMessage(c.PeerHostname, fmt.Sprintf("[failed to decrypt message: %v]", err))
		return
	}

	msg := Message{
		ID:        chatMsg.ID,
		Sender:    c.PeerHostname,
		Content:   string(plaintext),
		Timestamp: time.Unix(0, chatMsg.Timestamp),
		IsOwn:     false,
		State:     StateDelivered,
	}

	chatKey := c.PeerHostname
	m.mu.Lock()
	m.messages[chatKey] = append(m.messages[chatKey], msg)
	m.trimMessages(chatKey)
	m.unread[chatKey]++
	m.persistMessages(chatKey)
	m.mu.Unlock()

	ack := &protocol.Ack{MessageID: chatMsg.ID}
	ackEnv, wrapErr := protocol.Wrap(protocol.TypeAck, ack)
	if wrapErr == nil {
		c.WriteMessage(ackEnv)
	}

	if m.onMessage != nil {
		m.onMessage(chatKey, msg)
	}
}

func (m *Manager) handleAck(_ *tcnet.Connection, env *protocol.Envelope) {
	ack, err := protocol.Unwrap[protocol.Ack](env)
	if err != nil {
		return
	}
	m.mu.Lock()
	for chatKey, msgs := range m.messages {
		for i, msg := range msgs {
			if msg.ID == ack.MessageID && msg.State < StateDelivered {
				m.messages[chatKey][i].State = StateDelivered
				m.persistMessages(chatKey)
				break
			}
		}
	}
	m.mu.Unlock()
}

func (m *Manager) systemMessage(chatKey, text string) {
	msg := Message{
		ID:        uuid.New().String(),
		Sender:    "system",
		Content:   text,
		Timestamp: time.Now(),
		IsOwn:     false,
	}
	m.mu.Lock()
	m.messages[chatKey] = append(m.messages[chatKey], msg)
	m.trimMessages(chatKey)
	m.mu.Unlock()
	if m.onMessage != nil {
		m.onMessage(chatKey, msg)
	}
}

func (m *Manager) handlePing(c *tcnet.Connection) {
	pong := &protocol.Pong{Timestamp: time.Now().UnixNano()}
	env, err := protocol.Wrap(protocol.TypePong, pong)
	if err != nil {
		return
	}
	c.WriteMessage(env)
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

func (m *Manager) handleGroupAccept(_ *tcnet.Connection, env *protocol.Envelope) {
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
		Sender:    c.PeerHostname, // Use authenticated peer hostname, not self-reported sender
		Content:   string(plaintext),
		Timestamp: time.Unix(0, groupMsg.Timestamp),
		IsOwn:     false,
		GroupID:   groupMsg.GroupID,
		State:     StateDelivered,
	}

	chatKey := "group:" + groupMsg.GroupID
	m.mu.Lock()
	m.messages[chatKey] = append(m.messages[chatKey], msg)
	m.trimMessages(chatKey)
	m.unread[chatKey]++
	m.persistMessages(chatKey)
	m.mu.Unlock()

	if m.onMessage != nil {
		m.onMessage(chatKey, msg)
	}
}

func (m *Manager) handleTyping(c *tcnet.Connection, env *protocol.Envelope) {
	typing, err := protocol.Unwrap[protocol.Typing](env)
	if err != nil {
		return
	}
	chatKey := c.PeerHostname
	if strings.HasPrefix(typing.ChatKey, "group:") {
		chatKey = typing.ChatKey
	}
	m.mu.Lock()
	if typing.IsTyping {
		m.typing[chatKey] = time.Now()
	} else {
		delete(m.typing, chatKey)
	}
	m.mu.Unlock()
	if m.onTyping != nil {
		m.onTyping(chatKey, typing.IsTyping)
	}
}

func (m *Manager) handleReaction(c *tcnet.Connection, env *protocol.Envelope) {
	reaction, err := protocol.Unwrap[protocol.Reaction](env)
	if err != nil {
		return
	}
	m.mu.Lock()
	chatKey := c.PeerHostname
	if strings.HasPrefix(reaction.ChatKey, "group:") {
		chatKey = reaction.ChatKey
	}
	for i, msg := range m.messages[chatKey] {
		if msg.ID == reaction.MessageID {
			if reaction.Remove {
				var kept []Reaction
				for _, r := range m.messages[chatKey][i].Reactions {
					if !(r.Emoji == reaction.Emoji && r.Sender == reaction.Sender) {
						kept = append(kept, r)
					}
				}
				m.messages[chatKey][i].Reactions = kept
			} else {
				m.messages[chatKey][i].Reactions = append(m.messages[chatKey][i].Reactions, Reaction{
					Emoji:  reaction.Emoji,
					Sender: reaction.Sender,
				})
			}
			m.persistMessages(chatKey)
			break
		}
	}
	m.mu.Unlock()
	if m.onReaction != nil {
		m.onReaction(chatKey, reaction.MessageID)
	}
}

func (m *Manager) handleStatusMsg(c *tcnet.Connection, env *protocol.Envelope) {
	status, err := protocol.Unwrap[protocol.Status](env)
	if err != nil {
		return
	}
	m.mu.Lock()
	m.peerStatus[c.PeerHostname] = status.State
	m.mu.Unlock()
	if m.onStatus != nil {
		m.onStatus(c.PeerHostname, status.State)
	}
}

func (m *Manager) handleReadReceipt(c *tcnet.Connection, env *protocol.Envelope) {
	receipt, err := protocol.Unwrap[protocol.ReadReceipt](env)
	if err != nil {
		return
	}
	m.mu.Lock()
	chatKey := c.PeerHostname
	if strings.HasPrefix(receipt.ChatKey, "group:") {
		chatKey = receipt.ChatKey
	}
	for i, msg := range m.messages[chatKey] {
		if msg.ID == receipt.MessageID {
			m.messages[chatKey][i].State = StateRead
			m.persistMessages(chatKey)
			break
		}
	}
	m.mu.Unlock()
}

// Public API

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

	msg := Message{
		ID:        chatMsg.ID,
		Sender:    m.hostname,
		Content:   content,
		Timestamp: now,
		IsOwn:     true,
		State:     StateSending,
	}
	m.mu.Lock()
	m.messages[peerHostname] = append(m.messages[peerHostname], msg)
	m.trimMessages(peerHostname)
	m.persistMessages(peerHostname)
	m.mu.Unlock()

	if m.onMessage != nil {
		m.onMessage(peerHostname, msg)
	}
	return nil
}

func (m *Manager) SendTyping(chatKey string, isTyping bool) {
	if strings.HasPrefix(chatKey, "group:") {
		return // no typing indicators for groups
	}
	go func() {
		conn := m.server.GetConnection(chatKey)
		if conn == nil {
			return
		}
		env, err := protocol.Wrap(protocol.TypeTyping, &protocol.Typing{ChatKey: chatKey, IsTyping: isTyping})
		if err != nil {
			return
		}
		conn.WriteMessage(env)
	}()
}

func (m *Manager) SendReaction(chatKey, messageID, emoji string) {
	reaction := &protocol.Reaction{
		MessageID: messageID,
		ChatKey:   chatKey,
		Emoji:     emoji,
		Sender:    m.hostname,
	}

	m.mu.Lock()
	for i, msg := range m.messages[chatKey] {
		if msg.ID == messageID {
			m.messages[chatKey][i].Reactions = append(m.messages[chatKey][i].Reactions, Reaction{
				Emoji: emoji, Sender: m.hostname,
			})
			m.persistMessages(chatKey)
			break
		}
	}
	m.mu.Unlock()

	// Send over the network asynchronously to avoid blocking the UI
	go m.sendReactionToNetwork(chatKey, reaction)

	if m.onReaction != nil {
		m.onReaction(chatKey, messageID)
	}
}

func (m *Manager) sendReactionToNetwork(chatKey string, reaction *protocol.Reaction) {
	env, err := protocol.Wrap(protocol.TypeReaction, reaction)
	if err != nil {
		return
	}

	if strings.HasPrefix(chatKey, "group:") {
		groupID := strings.TrimPrefix(chatKey, "group:")
		m.mu.RLock()
		group := m.groups[groupID]
		m.mu.RUnlock()
		if group != nil {
			for _, member := range group.Members {
				if member == m.hostname {
					continue
				}
				conn := m.server.GetConnection(member)
				if conn != nil {
					conn.WriteMessage(env)
				}
			}
		}
	} else {
		conn := m.server.GetConnection(chatKey)
		if conn != nil {
			conn.WriteMessage(env)
		}
	}
}

func (m *Manager) SendReadReceipts(chatKey string) {
	m.mu.Lock()
	var toSend []string
	for i, msg := range m.messages[chatKey] {
		if !msg.IsOwn && msg.State < StateRead && msg.Sender != "system" {
			m.messages[chatKey][i].State = StateRead
			toSend = append(toSend, msg.ID)
		}
	}
	if len(toSend) > 0 {
		m.persistMessages(chatKey)
	}
	m.mu.Unlock()

	if strings.HasPrefix(chatKey, "group:") {
		return
	}
	conn := m.server.GetConnection(chatKey)
	if conn == nil {
		return
	}
	for _, id := range toSend {
		env, err := protocol.Wrap(protocol.TypeReadReceipt, &protocol.ReadReceipt{MessageID: id, ChatKey: chatKey})
		if err != nil {
			continue
		}
		conn.WriteMessage(env)
	}
}

func (m *Manager) SetStatus(state string) {
	m.mu.Lock()
	m.peerStatus[m.hostname] = state
	m.mu.Unlock()
	env, err := protocol.Wrap(protocol.TypeStatus, &protocol.Status{State: state})
	if err != nil {
		return
	}
	for _, conn := range m.server.Connections() {
		conn.WriteMessage(env)
	}
}

func (m *Manager) GetStatus(hostname string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if s := m.peerStatus[hostname]; s != "" {
		return s
	}
	return "available"
}

func (m *Manager) IsTyping(chatKey string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.typing[chatKey]
	if !ok {
		return false
	}
	return time.Since(t) < 3*time.Second
}

// SendFile sends a file to a peer (or group) over the encrypted TCP connection.
// It creates a placeholder message immediately and streams the file data async.
// Returns the message ID for tracking.
func (m *Manager) SendFile(chatKey, filePath string) (string, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return "", fmt.Errorf("stat file: %w", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("cannot send a directory")
	}

	// Compute SHA-256 checksum
	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open file: %w", err)
	}
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		f.Close()
		return "", fmt.Errorf("hash file: %w", err)
	}
	f.Close()
	checksum := hex.EncodeToString(h.Sum(nil))

	msgID := uuid.New().String()
	transferID := uuid.New().String()
	msg := Message{
		ID:        msgID,
		Sender:    m.hostname,
		Content:   filepath.Base(filePath),
		Timestamp: time.Now(),
		IsOwn:     true,
		State:     StateSending,
		FileInfo: &FileInfo{
			Filename: filepath.Base(filePath),
			Size:     info.Size(),
			State:    FileSending,
		},
	}

	m.mu.Lock()
	m.messages[chatKey] = append(m.messages[chatKey], msg)
	m.trimMessages(chatKey)
	m.persistMessages(chatKey)
	m.mu.Unlock()

	if m.onMessage != nil {
		m.onMessage(chatKey, msg)
	}

	go m.sendFileData(chatKey, filePath, msgID, transferID, checksum, info.Size())

	return msgID, nil
}

func (m *Manager) sendFileData(chatKey, filePath, msgID, transferID, checksum string, size int64) {
	// Determine connections to send to
	var conns []*tcnet.Connection
	if strings.HasPrefix(chatKey, "group:") {
		groupID := strings.TrimPrefix(chatKey, "group:")
		m.mu.RLock()
		group := m.groups[groupID]
		m.mu.RUnlock()
		if group != nil {
			for _, member := range group.Members {
				if member == m.hostname {
					continue
				}
				if c := m.server.GetConnection(member); c != nil {
					conns = append(conns, c)
				}
			}
		}
	} else {
		if c := m.server.GetConnection(chatKey); c != nil {
			conns = append(conns, c)
		}
	}

	if len(conns) == 0 {
		m.updateFileSendState(chatKey, msgID, FileFailed, "not connected")
		return
	}

	// Send FileOffer
	offer := &protocol.FileOffer{
		ID:       transferID,
		Filename: filepath.Base(filePath),
		Size:     size,
		Checksum: checksum,
		ChatKey:  chatKey,
	}
	offerEnv, err := protocol.Wrap(protocol.TypeFileOffer, offer)
	if err != nil {
		m.updateFileSendState(chatKey, msgID, FileFailed, err.Error())
		return
	}
	for _, c := range conns {
		c.WriteMessage(offerEnv)
	}

	// Stream file chunks
	f, err := os.Open(filePath)
	if err != nil {
		m.updateFileSendState(chatKey, msgID, FileFailed, err.Error())
		return
	}
	defer f.Close()

	buf := make([]byte, fileChunkSize)
	var offset int64
	for {
		n, readErr := f.Read(buf)
		if n > 0 {
			encoded := base64.StdEncoding.EncodeToString(buf[:n])
			chunk := &protocol.FileData{
				ID:     transferID,
				Offset: offset,
				Data:   encoded,
			}
			chunkEnv, wErr := protocol.Wrap(protocol.TypeFileData, chunk)
			if wErr != nil {
				m.updateFileSendState(chatKey, msgID, FileFailed, wErr.Error())
				return
			}
			for _, c := range conns {
				if writeErr := c.WriteMessage(chunkEnv); writeErr != nil {
					m.updateFileSendState(chatKey, msgID, FileFailed, writeErr.Error())
					return
				}
			}
			offset += int64(n)
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			m.updateFileSendState(chatKey, msgID, FileFailed, readErr.Error())
			return
		}
	}

	// Send FileComplete
	complete := &protocol.FileComplete{ID: transferID}
	completeEnv, err := protocol.Wrap(protocol.TypeFileComplete, complete)
	if err != nil {
		m.updateFileSendState(chatKey, msgID, FileFailed, err.Error())
		return
	}
	for _, c := range conns {
		c.WriteMessage(completeEnv)
	}

	m.updateFileSendState(chatKey, msgID, FileSent, "")
}

func (m *Manager) updateFileSendState(chatKey, msgID string, state FileState, errMsg string) {
	m.mu.Lock()
	for i, msg := range m.messages[chatKey] {
		if msg.ID == msgID && msg.FileInfo != nil {
			m.messages[chatKey][i].FileInfo.State = state
			if errMsg != "" {
				m.messages[chatKey][i].FileInfo.Error = errMsg
			}
			if state == FileSent {
				m.messages[chatKey][i].State = StateDelivered
			}
			m.persistMessages(chatKey)
			break
		}
	}
	m.mu.Unlock()

	if m.onMessage != nil {
		m.onMessage(chatKey, Message{ID: msgID})
	}
}

// handleFileOffer processes an incoming file offer from a peer.
func (m *Manager) handleFileOffer(c *tcnet.Connection, env *protocol.Envelope) {
	offer, err := protocol.Unwrap[protocol.FileOffer](env)
	if err != nil {
		return
	}

	// Determine the chat key — use sender hostname for DMs
	chatKey := c.PeerHostname
	if strings.HasPrefix(offer.ChatKey, "group:") {
		chatKey = offer.ChatKey
	}

	// Create temp file for receiving data
	home, _ := os.UserHomeDir()
	dlDir := filepath.Join(home, "Downloads", "tailchat")
	os.MkdirAll(dlDir, 0700)

	tmpFile, err := os.CreateTemp(dlDir, ".tailchat-recv-*")
	if err != nil {
		m.systemMessage(chatKey, fmt.Sprintf("[file receive failed: %v]", err))
		return
	}

	msgID := uuid.New().String()
	msg := Message{
		ID:        msgID,
		Sender:    c.PeerHostname,
		Content:   offer.Filename,
		Timestamp: time.Now(),
		IsOwn:     false,
		State:     StateDelivered,
		FileInfo: &FileInfo{
			Filename: offer.Filename,
			Size:     offer.Size,
			State:    FileSending, // receiving in progress
		},
	}

	m.mu.Lock()
	m.activeTransfers[offer.ID] = &fileTransfer{
		chatKey:  chatKey,
		msgID:    msgID,
		filename: offer.Filename,
		size:     offer.Size,
		checksum: offer.Checksum,
		file:     tmpFile,
	}
	m.messages[chatKey] = append(m.messages[chatKey], msg)
	m.trimMessages(chatKey)
	m.unread[chatKey]++
	m.persistMessages(chatKey)
	m.mu.Unlock()

	if m.onMessage != nil {
		m.onMessage(chatKey, msg)
	}
}

// handleFileData processes an incoming chunk of file data.
func (m *Manager) handleFileData(_ *tcnet.Connection, env *protocol.Envelope) {
	data, err := protocol.Unwrap[protocol.FileData](env)
	if err != nil {
		return
	}

	m.mu.RLock()
	ft, ok := m.activeTransfers[data.ID]
	m.mu.RUnlock()
	if !ok {
		return
	}

	decoded, err := base64.StdEncoding.DecodeString(data.Data)
	if err != nil {
		return
	}

	if _, err := ft.file.WriteAt(decoded, data.Offset); err != nil {
		return
	}

	m.mu.Lock()
	ft.received += int64(len(decoded))
	m.mu.Unlock()
}

// handleFileComplete finalises an incoming file transfer.
func (m *Manager) handleFileComplete(_ *tcnet.Connection, env *protocol.Envelope) {
	complete, err := protocol.Unwrap[protocol.FileComplete](env)
	if err != nil {
		return
	}

	m.mu.Lock()
	ft, ok := m.activeTransfers[complete.ID]
	if !ok {
		m.mu.Unlock()
		return
	}
	delete(m.activeTransfers, complete.ID)
	m.mu.Unlock()

	ft.file.Close()

	// Verify checksum
	f, err := os.Open(ft.file.Name())
	if err != nil {
		m.updateFileRecvState(ft, FileFailed, "cannot reopen file")
		os.Remove(ft.file.Name())
		return
	}
	h := sha256.New()
	io.Copy(h, f)
	f.Close()
	got := hex.EncodeToString(h.Sum(nil))

	if got != ft.checksum {
		m.updateFileRecvState(ft, FileFailed, "checksum mismatch")
		os.Remove(ft.file.Name())
		return
	}

	// Move to final location
	home, _ := os.UserHomeDir()
	dlDir := filepath.Join(home, "Downloads", "tailchat")
	finalPath := filepath.Join(dlDir, ft.filename)

	// Conflict resolution: add suffix if file exists
	if _, err := os.Stat(finalPath); err == nil {
		ext := filepath.Ext(ft.filename)
		base := strings.TrimSuffix(ft.filename, ext)
		for i := 1; ; i++ {
			finalPath = filepath.Join(dlDir, fmt.Sprintf("%s(%d)%s", base, i, ext))
			if _, err := os.Stat(finalPath); os.IsNotExist(err) {
				break
			}
		}
	}

	if err := os.Rename(ft.file.Name(), finalPath); err != nil {
		m.updateFileRecvState(ft, FileFailed, err.Error())
		os.Remove(ft.file.Name())
		return
	}

	// Update message state
	m.mu.Lock()
	for i, msg := range m.messages[ft.chatKey] {
		if msg.ID == ft.msgID && msg.FileInfo != nil {
			m.messages[ft.chatKey][i].FileInfo.State = FileReceived
			m.messages[ft.chatKey][i].FileInfo.Path = finalPath
			m.persistMessages(ft.chatKey)
			break
		}
	}
	m.mu.Unlock()

	if m.onMessage != nil {
		m.onMessage(ft.chatKey, Message{ID: ft.msgID})
	}
}

func (m *Manager) updateFileRecvState(ft *fileTransfer, state FileState, errMsg string) {
	m.mu.Lock()
	for i, msg := range m.messages[ft.chatKey] {
		if msg.ID == ft.msgID && msg.FileInfo != nil {
			m.messages[ft.chatKey][i].FileInfo.State = state
			m.messages[ft.chatKey][i].FileInfo.Error = errMsg
			m.persistMessages(ft.chatKey)
			break
		}
	}
	m.mu.Unlock()

	if m.onMessage != nil {
		m.onMessage(ft.chatKey, Message{ID: ft.msgID})
	}
}

// UpdateFileState updates the FileInfo state of an existing message by ID.
func (m *Manager) UpdateFileState(chatKey, msgID string, state FileState, errMsg string) {
	m.mu.Lock()
	for i, msg := range m.messages[chatKey] {
		if msg.ID == msgID && msg.FileInfo != nil {
			m.messages[chatKey][i].FileInfo.State = state
			if errMsg != "" {
				m.messages[chatKey][i].FileInfo.Error = errMsg
			}
			m.persistMessages(chatKey)
			break
		}
	}
	m.mu.Unlock()
}

func (m *Manager) ConnectToPeer(ip string) (*tcnet.Connection, error) {
	addr := fmt.Sprintf("%s:%d", ip, tcnet.DefaultPort)
	conn, err := tcnet.Connect(addr, m.keyPair, m.hostname, m.knownKeys)
	if err != nil {
		return nil, err
	}
	m.server.AddConnection(conn)
	m.mu.Lock()
	m.reconnectTargets[conn.PeerHostname] = ip
	m.mu.Unlock()
	return conn, nil
}

func (m *Manager) reconnectLoop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			m.mu.RLock()
			targets := make(map[string]string, len(m.reconnectTargets))
			for k, v := range m.reconnectTargets {
				targets[k] = v
			}
			m.mu.RUnlock()
			for hostname, ip := range targets {
				if m.server.GetConnection(hostname) != nil {
					continue
				}
				addr := fmt.Sprintf("%s:%d", ip, tcnet.DefaultPort)
				conn, err := tcnet.Connect(addr, m.keyPair, m.hostname, m.knownKeys)
				if err != nil {
					continue
				}
				m.server.AddConnection(conn)
			}
		case <-m.reconnectStop:
			return
		}
	}
}

func (m *Manager) Stop() {
	m.stopOnce.Do(func() {
		close(m.reconnectStop)
		close(m.persistCh)
	})
}

func (m *Manager) SearchMessages(query string) map[string][]Message {
	if m.store == nil {
		results := make(map[string][]Message)
		m.mu.RLock()
		lq := strings.ToLower(query)
		for chatKey, msgs := range m.messages {
			for _, msg := range msgs {
				if strings.Contains(strings.ToLower(msg.Content), lq) {
					results[chatKey] = append(results[chatKey], msg)
				}
			}
		}
		m.mu.RUnlock()
		return results
	}
	stored, err := m.store.Search(query)
	if err != nil {
		return nil
	}
	results := make(map[string][]Message)
	for chatKey, smsgs := range stored {
		for _, sm := range smsgs {
			results[chatKey] = append(results[chatKey], Message{
				ID: sm.ID, Sender: sm.Sender, Content: sm.Content,
				Timestamp: sm.Timestamp, IsOwn: sm.IsOwn, GroupID: sm.GroupID,
			})
		}
	}
	return results
}

func (m *Manager) GetMessages(chatKey string) []Message {
	m.mu.RLock()
	defer m.mu.RUnlock()
	msgs := m.messages[chatKey]
	result := make([]Message, len(msgs))
	for i, msg := range msgs {
		result[i] = msg
		if len(msg.Reactions) > 0 {
			result[i].Reactions = make([]Reaction, len(msg.Reactions))
			copy(result[i].Reactions, msg.Reactions)
		}
		if msg.FileInfo != nil {
			fi := *msg.FileInfo
			result[i].FileInfo = &fi
		}
	}
	return result
}

func (m *Manager) CreateGroup(name string, memberHostnames []string) (*Group, error) {
	// Copy the slice to avoid mutating the caller's underlying array.
	members := make([]string, len(memberHostnames)+1)
	copy(members, memberHostnames)
	members[len(memberHostnames)] = m.hostname
	group := &Group{
		ID:      uuid.New().String(),
		Name:    name,
		Members: members,
	}
	m.mu.Lock()
	m.groups[group.ID] = group
	m.persistGroups()
	m.mu.Unlock()

	invite := &protocol.GroupInvite{GroupID: group.ID, GroupName: name, Members: group.Members}
	for _, host := range memberHostnames {
		conn := m.server.GetConnection(host)
		if conn == nil {
			continue
		}
		tcnet.SendEncrypted(conn, protocol.TypeGroupInvite, invite)
	}
	return group, nil
}

func (m *Manager) AcceptGroupInvite(invite *protocol.GroupInvite, fromHost string) {
	group := &Group{ID: invite.GroupID, Name: invite.GroupName, Members: invite.Members}
	m.mu.Lock()
	if _, exists := m.groups[group.ID]; exists {
		// Duplicate invite — already accepted this group.
		m.mu.Unlock()
		return
	}
	m.groups[group.ID] = group
	m.persistGroups()
	m.mu.Unlock()

	conn := m.server.GetConnection(fromHost)
	if conn != nil {
		accept := &protocol.GroupAccept{GroupID: group.ID}
		tcnet.SendEncrypted(conn, protocol.TypeGroupAccept, accept)
	}
}

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
			ID: msgID, GroupID: groupID, Sender: m.hostname,
			Ciphertext: ciphertext, Timestamp: now.UnixNano(),
		}
		tcnet.SendEncrypted(conn, protocol.TypeGroupChat, groupMsg)
	}

	msg := Message{
		ID: msgID, Sender: m.hostname, Content: content,
		Timestamp: now, IsOwn: true, GroupID: groupID, State: StateSending,
	}
	chatKey := "group:" + groupID
	m.mu.Lock()
	m.messages[chatKey] = append(m.messages[chatKey], msg)
	m.trimMessages(chatKey)
	m.persistMessages(chatKey)
	m.mu.Unlock()

	if m.onMessage != nil {
		m.onMessage(chatKey, msg)
	}
	return nil
}

func (m *Manager) Groups() []*Group {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var groups []*Group
	for _, g := range m.groups {
		groups = append(groups, g)
	}
	return groups
}

func (m *Manager) Unread(chatKey string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.unread[chatKey]
}

func (m *Manager) ClearUnread(chatKey string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.unread, chatKey)
}

func (m *Manager) IsConnected(hostname string) bool {
	return m.server.GetConnection(hostname) != nil
}
