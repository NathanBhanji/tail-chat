package main

import (
	"fmt"
	"os"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/NathanBhanji/tail-chat/internal/chat"
	"github.com/NathanBhanji/tail-chat/internal/crypto"
	"github.com/NathanBhanji/tail-chat/internal/discovery"
	tcnet "github.com/NathanBhanji/tail-chat/internal/net"
	"github.com/NathanBhanji/tail-chat/internal/protocol"
	"github.com/NathanBhanji/tail-chat/internal/storage"
	"github.com/NathanBhanji/tail-chat/internal/tui"
)

var (
	version = "dev"
	commit  = "none"
)

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Printf("tailchat %s (%s)\n", version, commit)
		return
	}

	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "tailchat: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Load or generate identity
	kp, err := crypto.LoadOrGenerateKeyPair()
	if err != nil {
		return fmt.Errorf("identity: %w", err)
	}

	// Get our Tailscale info
	selfIP, selfHost, err := discovery.GetSelfIP()
	if err != nil {
		return fmt.Errorf("tailscale not running? %w", err)
	}

	// Load TOFU known keys
	knownKeys, err := crypto.LoadKnownKeys()
	if err != nil {
		return fmt.Errorf("known keys: %w", err)
	}

	// Start TCP server
	listenAddr := fmt.Sprintf("%s:%d", selfIP, tcnet.DefaultPort)
	server, err := tcnet.NewServer(listenAddr, kp, selfHost, knownKeys)
	if err != nil {
		return fmt.Errorf("server: %w", err)
	}
	server.Start()
	defer server.Stop()

	// Create bubbletea program (we need it for sending messages to the TUI).
	// Use atomic.Pointer to avoid data race between main goroutine and callbacks.
	var programPtr atomic.Pointer[tea.Program]

	// sendToProgram safely sends a message to the TUI if it's ready.
	sendToProgram := func(msg tea.Msg) {
		if p := programPtr.Load(); p != nil {
			p.Send(msg)
		}
	}

	// Start peer discovery
	watcher := discovery.NewWatcher(10*time.Second, func(peers []discovery.Peer) {
		sendToProgram(tui.PeersUpdatedMsg{Peers: peers})
	})
	if err := watcher.Start(); err != nil {
		return fmt.Errorf("peer discovery: %w", err)
	}
	defer watcher.Stop()

	// Wire hostname verification: resolve Tailscale IP -> authenticated hostname.
	server.SetHostnameResolver(func(ip string) string {
		for _, p := range watcher.Peers() {
			if p.TailscaleIP == ip {
				return p.Hostname
			}
		}
		return "" // unknown IP — allow (may be a new peer not yet in discovery)
	})

	// Create persistence store
	store, err := storage.New("")
	if err != nil {
		return fmt.Errorf("storage: %w", err)
	}

	// Create chat manager
	chatMgr := chat.NewManager(server, kp, selfHost, store, knownKeys)
	defer chatMgr.Stop()

	// Wire message callbacks to TUI
	chatMgr.OnMessage(func(chatKey string, msg chat.Message) {
		sendToProgram(tui.IncomingMsg{ChatKey: chatKey, Message: msg})
	})

	chatMgr.OnGroupInvite(func(invite *protocol.GroupInvite, from string) {
		sendToProgram(tui.GroupInviteMsg{Invite: invite, From: from})
	})

	chatMgr.OnPeerConnect(func(hostname string) {
		sendToProgram(tui.PeerConnectMsg{Hostname: hostname})
	})

	chatMgr.OnTyping(func(chatKey string, isTyping bool) {
		sendToProgram(tui.TypingMsg{ChatKey: chatKey, IsTyping: isTyping})
	})

	chatMgr.OnReaction(func(chatKey string, msgID string) {
		sendToProgram(tui.ReactionUpdatedMsg{ChatKey: chatKey})
	})

	chatMgr.OnStatus(func(hostname string, state string) {
		sendToProgram(tui.StatusUpdatedMsg{Hostname: hostname, State: state})
	})

	// Create and run TUI
	model := tui.NewModel(chatMgr, watcher)
	p := tea.NewProgram(model, tea.WithAltScreen())
	programPtr.Store(p)

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("tui: %w", err)
	}

	return nil
}
