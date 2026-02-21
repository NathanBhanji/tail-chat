package main

import (
	"context"
	"fmt"
	"log"
	"sync/atomic"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/NathanBhanji/tail-chat/internal/chat"
	"github.com/NathanBhanji/tail-chat/internal/config"
	"github.com/NathanBhanji/tail-chat/internal/crypto"
	"github.com/NathanBhanji/tail-chat/internal/discovery"
	tcnet "github.com/NathanBhanji/tail-chat/internal/net"
	"github.com/NathanBhanji/tail-chat/internal/protocol"
	"github.com/NathanBhanji/tail-chat/internal/storage"
	"github.com/NathanBhanji/tail-chat/internal/tenor"
)

// App is the Wails application struct. All exported methods are
// automatically bound and callable from the frontend via wailsjs.
type App struct {
	ctx     context.Context
	chatMgr *chat.Manager
	watcher *discovery.Watcher
	server  *tcnet.Server
	tenor   *tenor.Client

	selfHost   string
	selfIP     string
	ready      atomic.Bool
	initErr    error
	frontendUp chan struct{} // closed when frontend calls NotifyFrontendReady
}

// NewApp creates a new App instance.
func NewApp() *App {
	return &App{
		tenor:      tenor.New(),
		frontendUp: make(chan struct{}),
	}
}

// startup is called by Wails when the application starts.
// We start the backend init early but don't emit events yet —
// the webview DOM may not be ready to receive them.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	go func() {
		log.Println("[tailchat] starting backend init...")
		if err := a.initBackend(); err != nil {
			log.Printf("[tailchat] backend init failed: %v", err)
			a.initErr = err
			// Wait for frontend before emitting error
			<-a.frontendUp
			wailsRuntime.EventsEmit(a.ctx, "error", err.Error())
			return
		}
		log.Println("[tailchat] backend ready")
		a.ready.Store(true)
		// Wait until the frontend has registered its event listeners
		<-a.frontendUp
		log.Println("[tailchat] frontend ready, emitting ready event")
		wailsRuntime.EventsEmit(a.ctx, "ready", true)
	}()
}

// NotifyFrontendReady is called by the frontend JS once it has
// registered all event listeners. This handshake guarantees the
// "ready" event is never emitted before the frontend can receive it.
func (a *App) NotifyFrontendReady() {
	select {
	case <-a.frontendUp:
		// already closed
	default:
		close(a.frontendUp)
	}
}

// shutdown is called by Wails when the application is closing.
func (a *App) shutdown(ctx context.Context) {
	if a.chatMgr != nil {
		a.chatMgr.Stop()
	}
	if a.watcher != nil {
		a.watcher.Stop()
	}
	if a.server != nil {
		a.server.Stop()
	}
}

func (a *App) initBackend() error {
	kp, err := crypto.LoadOrGenerateKeyPair()
	if err != nil {
		return fmt.Errorf("identity: %w", err)
	}

	selfIP, selfHost, err := discovery.GetSelfIP()
	if err != nil {
		return fmt.Errorf("tailscale not running? %w", err)
	}
	a.selfIP = selfIP
	a.selfHost = selfHost

	knownKeys, err := crypto.LoadKnownKeys()
	if err != nil {
		return fmt.Errorf("known keys: %w", err)
	}

	listenAddr := fmt.Sprintf("%s:%d", selfIP, tcnet.DefaultPort)
	server, err := tcnet.NewServer(listenAddr, kp, selfHost, knownKeys)
	if err != nil {
		return fmt.Errorf("server: %w", err)
	}
	server.Start()
	a.server = server

	watcher := discovery.NewWatcher(10*time.Second, func(peers []discovery.Peer) {
		wailsRuntime.EventsEmit(a.ctx, "peers:updated", peers)
	})
	if err := watcher.Start(); err != nil {
		return fmt.Errorf("peer discovery: %w", err)
	}
	a.watcher = watcher

	server.SetHostnameResolver(func(ip string) string {
		for _, p := range watcher.Peers() {
			if p.TailscaleIP == ip {
				return p.Hostname
			}
		}
		return ""
	})

	store, err := storage.New("")
	if err != nil {
		return fmt.Errorf("storage: %w", err)
	}

	chatMgr := chat.NewManager(server, kp, selfHost, store, knownKeys)
	a.chatMgr = chatMgr

	// Bridge all chat callbacks to Wails events
	chatMgr.OnMessage(func(chatKey string, msg chat.Message) {
		wailsRuntime.EventsEmit(a.ctx, "chat:message", map[string]interface{}{
			"chatKey": chatKey,
			"message": msg,
		})
	})
	chatMgr.OnGroupInvite(func(invite *protocol.GroupInvite, from string) {
		wailsRuntime.EventsEmit(a.ctx, "chat:groupInvite", map[string]interface{}{
			"invite": invite,
			"from":   from,
		})
	})
	chatMgr.OnPeerConnect(func(hostname string) {
		wailsRuntime.EventsEmit(a.ctx, "chat:peerConnect", hostname)
	})
	chatMgr.OnTyping(func(chatKey string, isTyping bool) {
		wailsRuntime.EventsEmit(a.ctx, "chat:typing", map[string]interface{}{
			"chatKey":  chatKey,
			"isTyping": isTyping,
		})
	})
	chatMgr.OnReaction(func(chatKey string, msgID string) {
		wailsRuntime.EventsEmit(a.ctx, "chat:reaction", map[string]interface{}{
			"chatKey":   chatKey,
			"messageID": msgID,
		})
	})
	chatMgr.OnStatus(func(hostname string, state string) {
		wailsRuntime.EventsEmit(a.ctx, "chat:status", map[string]interface{}{
			"hostname": hostname,
			"state":    state,
		})
	})

	return nil
}

// ─── Peer discovery bindings ────────────────────────────────────────

// GetPeers returns the current list of Tailscale peers.
func (a *App) GetPeers() []discovery.Peer {
	if a.watcher == nil {
		return nil
	}
	return a.watcher.Peers()
}

// IsReady returns whether the backend has finished initialising.
// The frontend polls this on mount to avoid missing the one-shot "ready" event.
func (a *App) IsReady() bool {
	return a.ready.Load()
}

// GetSelfInfo returns hostname and IP for the local node.
func (a *App) GetSelfInfo() map[string]string {
	return map[string]string{
		"hostname": a.selfHost,
		"ip":       a.selfIP,
	}
}

// ─── Chat bindings ──────────────────────────────────────────────────

// GetMessages returns messages for a given chat key.
func (a *App) GetMessages(chatKey string) []chat.Message {
	if a.chatMgr == nil {
		return nil
	}
	a.chatMgr.LoadMessages(chatKey)
	return a.chatMgr.GetMessages(chatKey)
}

// SendMessage sends a chat message to a peer.
func (a *App) SendMessage(peerHostname, content string) error {
	if a.chatMgr == nil {
		return fmt.Errorf("not ready")
	}
	return a.chatMgr.SendMessage(peerHostname, content)
}

// SendGroupMessage sends a message to a group chat.
func (a *App) SendGroupMessage(groupID, content string) error {
	if a.chatMgr == nil {
		return fmt.Errorf("not ready")
	}
	return a.chatMgr.SendGroupMessage(groupID, content)
}

// SendTyping notifies the peer that we are typing.
func (a *App) SendTyping(chatKey string, isTyping bool) {
	if a.chatMgr != nil {
		a.chatMgr.SendTyping(chatKey, isTyping)
	}
}

// SendReaction sends an emoji reaction to a message.
func (a *App) SendReaction(chatKey, messageID, emoji string) {
	if a.chatMgr != nil {
		a.chatMgr.SendReaction(chatKey, messageID, emoji)
	}
}

// SendReadReceipts marks messages as read for a chat.
func (a *App) SendReadReceipts(chatKey string) {
	if a.chatMgr != nil {
		a.chatMgr.SendReadReceipts(chatKey)
	}
}

// ConnectToPeer establishes a connection to a peer by IP.
// hostname is the expected Tailscale hostname for verification.
func (a *App) ConnectToPeer(ip, hostname string) error {
	if a.chatMgr == nil {
		return fmt.Errorf("not ready")
	}
	_, err := a.chatMgr.ConnectToPeer(ip, hostname)
	return err
}

// IsConnected checks if we have an active connection to a peer.
func (a *App) IsConnected(hostname string) bool {
	if a.chatMgr == nil {
		return false
	}
	return a.chatMgr.IsConnected(hostname)
}

// GetUnread returns unread count for a chat.
func (a *App) GetUnread(chatKey string) int {
	if a.chatMgr == nil {
		return 0
	}
	return a.chatMgr.Unread(chatKey)
}

// ClearUnread resets unread count for a chat.
func (a *App) ClearUnread(chatKey string) {
	if a.chatMgr != nil {
		a.chatMgr.ClearUnread(chatKey)
	}
}

// GetPeerStatus returns the status of a peer.
func (a *App) GetPeerStatus(hostname string) string {
	if a.chatMgr == nil {
		return "unknown"
	}
	return a.chatMgr.GetStatus(hostname)
}

// SetStatus sets our status visible to other peers.
func (a *App) SetStatus(state string) {
	if a.chatMgr != nil {
		a.chatMgr.SetStatus(state)
	}
}

// SearchMessages searches across all chats.
func (a *App) SearchMessages(query string) map[string][]chat.Message {
	if a.chatMgr == nil {
		return nil
	}
	return a.chatMgr.SearchMessages(query)
}

// ─── Group bindings ─────────────────────────────────────────────────

// CreateGroup creates a new group with the given members.
func (a *App) CreateGroup(name string, members []string) (*chat.Group, error) {
	if a.chatMgr == nil {
		return nil, fmt.Errorf("not ready")
	}
	return a.chatMgr.CreateGroup(name, members)
}

// GetGroups returns all groups.
func (a *App) GetGroups() []*chat.Group {
	if a.chatMgr == nil {
		return nil
	}
	return a.chatMgr.Groups()
}

// ─── Tenor GIF bindings ────────────────────────────────────────────

// SearchGifs searches Tenor for GIFs.
func (a *App) SearchGifs(query string, limit int) ([]tenor.GIF, error) {
	return a.tenor.Search(query, limit)
}

// TrendingGifs returns trending GIFs from Tenor.
func (a *App) TrendingGifs(limit int) ([]tenor.GIF, error) {
	return a.tenor.Trending(limit)
}

// ─── Theme bindings ─────────────────────────────────────────────────

// ListThemes returns all available themes.
func (a *App) ListThemes() []config.ThemeInfo {
	return config.ListThemes()
}

// GetActiveTheme returns the currently active theme name.
func (a *App) GetActiveTheme() string {
	return config.Load().ActiveTheme
}

// SetTheme sets the active theme and reloads the window.
func (a *App) SetTheme(name string) error {
	cfg := config.Load()
	cfg.ActiveTheme = name
	if err := config.Save(cfg); err != nil {
		return err
	}
	// Reload the webview to pick up the new theme
	wailsRuntime.WindowReloadApp(a.ctx)
	return nil
}
