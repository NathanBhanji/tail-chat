package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/NathanBhanji/tail-chat/internal/chat"
	"github.com/NathanBhanji/tail-chat/internal/crypto"
	"github.com/NathanBhanji/tail-chat/internal/discovery"
	tcnet "github.com/NathanBhanji/tail-chat/internal/net"
	"github.com/NathanBhanji/tail-chat/internal/protocol"
	"github.com/NathanBhanji/tail-chat/internal/storage"
)

// App is the Wails bridge struct. Its public methods are callable from JavaScript.
type App struct {
	ctx         context.Context
	chatMgr     *chat.Manager
	peerWatcher *discovery.Watcher
	server      *tcnet.Server
	keyPair     *crypto.KeyPair
	selfHost    string
	selfIP      string
	startupErr  string
}

// NewApp creates a new App instance.
func NewApp() *App {
	return &App{}
}

// startup is called by Wails when the app starts. We only save the context here.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// domReady is called after the frontend DOM is loaded, so events won't be missed.
func (a *App) domReady(ctx context.Context) {
	kp, err := crypto.LoadOrGenerateKeyPair()
	if err != nil {
		a.startupErr = fmt.Sprintf("Identity error: %v", err)
		runtime.EventsEmit(a.ctx, "error", a.startupErr)
		return
	}
	a.keyPair = kp
	log.Printf("[DEBUG] Loaded identity - public key: %s", kp.PublicKeyHex())

	selfIP, selfHost, err := discovery.GetSelfIP()
	if err != nil {
		a.startupErr = fmt.Sprintf("Tailscale not running? %v", err)
		runtime.EventsEmit(a.ctx, "error", a.startupErr)
		return
	}
	a.selfIP = selfIP
	a.selfHost = selfHost

	// Load TOFU known keys
	knownKeys, err := crypto.LoadKnownKeys()
	if err != nil {
		a.startupErr = fmt.Sprintf("Known keys error: %v", err)
		runtime.EventsEmit(a.ctx, "error", a.startupErr)
		return
	}

	listenAddr := fmt.Sprintf("%s:%d", selfIP, tcnet.DefaultPort)
	server, err := tcnet.NewServer(listenAddr, kp, selfHost, knownKeys)
	if err != nil {
		a.startupErr = fmt.Sprintf("Server error: %v", err)
		runtime.EventsEmit(a.ctx, "error", a.startupErr)
		return
	}
	server.Start()
	a.server = server

	// Start peer discovery (before hostname resolver so we have peers available)
	watcher := discovery.NewWatcher(10*time.Second, func(peers []discovery.Peer) {
		runtime.EventsEmit(a.ctx, "peers:updated", peers)
	})
	if err := watcher.Start(); err != nil {
		a.startupErr = fmt.Sprintf("Peer discovery error: %v", err)
		runtime.EventsEmit(a.ctx, "error", a.startupErr)
		return
	}
	a.peerWatcher = watcher

	// Wire hostname resolver: Tailscale IP -> authenticated hostname
	server.SetHostnameResolver(func(ip string) string {
		for _, p := range watcher.Peers() {
			if p.TailscaleIP == ip {
				return p.Hostname
			}
		}
		return ""
	})

	// Create persistence store
	store, err := storage.New("")
	if err != nil {
		a.startupErr = fmt.Sprintf("Storage error: %v", err)
		runtime.EventsEmit(a.ctx, "error", a.startupErr)
		return
	}

	a.chatMgr = chat.NewManager(server, kp, selfHost, store, knownKeys)

	a.chatMgr.OnMessage(func(chatKey string, msg chat.Message) {
		runtime.EventsEmit(a.ctx, "chat:message", map[string]interface{}{
			"chatKey":   chatKey,
			"id":        msg.ID,
			"sender":    msg.Sender,
			"content":   msg.Content,
			"timestamp": msg.Timestamp.Format(time.RFC3339),
			"isOwn":     msg.IsOwn,
			"groupID":   msg.GroupID,
		})
	})

	a.chatMgr.OnGroupInvite(func(invite *protocol.GroupInvite, from string) {
		runtime.EventsEmit(a.ctx, "chat:groupInvite", map[string]interface{}{
			"groupID":   invite.GroupID,
			"groupName": invite.GroupName,
			"members":   invite.Members,
			"from":      from,
		})
	})

	runtime.EventsEmit(a.ctx, "app:ready", map[string]string{
		"hostname": selfHost,
		"ip":       selfIP,
	})
}

// shutdown is called by Wails when the app closes.
func (a *App) shutdown(ctx context.Context) {
	if a.chatMgr != nil {
		a.chatMgr.Stop()
	}
	if a.peerWatcher != nil {
		a.peerWatcher.Stop()
	}
	if a.server != nil {
		a.server.Stop()
	}
}

// GetSelfInfo returns the local node's hostname and IP.
func (a *App) GetSelfInfo() map[string]string {
	return map[string]string{"hostname": a.selfHost, "ip": a.selfIP}
}

// GetPeers returns the current list of tailnet peers.
func (a *App) GetPeers() []discovery.Peer {
	if a.peerWatcher == nil {
		return nil
	}
	return a.peerWatcher.Peers()
}

// ConnectToPeer dials a peer by Tailscale IP.
func (a *App) ConnectToPeer(ip string) error {
	conn, err := a.chatMgr.ConnectToPeer(ip)
	if err != nil {
		return err
	}
	log.Printf("[DEBUG] Connected to %s - peer pubkey: %s, shared secret: %s",
		conn.PeerHostname,
		hex.EncodeToString(conn.PeerPublicKey[:]),
		hex.EncodeToString(conn.SharedSecret[:8]))
	return nil
}

// SendMessage sends an encrypted 1:1 message.
func (a *App) SendMessage(peerHostname, content string) error {
	return a.chatMgr.SendMessage(peerHostname, content)
}

// SendGroupMessage sends an encrypted message to a group.
func (a *App) SendGroupMessage(groupID, content string) error {
	return a.chatMgr.SendGroupMessage(groupID, content)
}

// JSMessage is a frontend-friendly message with timestamp as string.
type JSMessage struct {
	ID        string `json:"id"`
	Sender    string `json:"sender"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
	IsOwn     bool   `json:"isOwn"`
	GroupID   string `json:"groupID"`
}

// GetMessages returns chat history for a given chat key.
func (a *App) GetMessages(chatKey string) []JSMessage {
	a.chatMgr.LoadMessages(chatKey)
	msgs := a.chatMgr.GetMessages(chatKey)
	result := make([]JSMessage, len(msgs))
	for i, m := range msgs {
		result[i] = JSMessage{
			ID:        m.ID,
			Sender:    m.Sender,
			Content:   m.Content,
			Timestamp: m.Timestamp.Format(time.RFC3339),
			IsOwn:     m.IsOwn,
			GroupID:   m.GroupID,
		}
	}
	return result
}

// IsConnected checks if we have an active connection to a peer.
func (a *App) IsConnected(hostname string) bool {
	return a.chatMgr.IsConnected(hostname)
}

// GetGroups returns all active groups.
func (a *App) GetGroups() []*chat.Group {
	return a.chatMgr.Groups()
}

// CreateGroup creates a new group and invites members.
func (a *App) CreateGroup(name string, members []string) error {
	_, err := a.chatMgr.CreateGroup(name, members)
	return err
}

// AcceptGroupInvite accepts a pending group invitation.
func (a *App) AcceptGroupInvite(groupID, groupName string, members []string, fromHost string) {
	invite := &protocol.GroupInvite{
		GroupID:   groupID,
		GroupName: groupName,
		Members:   members,
	}
	a.chatMgr.AcceptGroupInvite(invite, fromHost)
}
