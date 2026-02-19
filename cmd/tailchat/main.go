package main

import (
	"fmt"
	"os"
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

	// Start TCP server
	listenAddr := fmt.Sprintf("%s:%d", selfIP, tcnet.DefaultPort)
	server, err := tcnet.NewServer(listenAddr, kp, selfHost)
	if err != nil {
		return fmt.Errorf("server: %w", err)
	}
	server.Start()
	defer server.Stop()

	// Create bubbletea program (we need it for sending messages to the TUI)
	var program *tea.Program

	// Start peer discovery
	watcher := discovery.NewWatcher(10*time.Second, func(peers []discovery.Peer) {
		if program != nil {
			program.Send(tui.PeersUpdatedMsg{Peers: peers})
		}
	})
	if err := watcher.Start(); err != nil {
		return fmt.Errorf("peer discovery: %w", err)
	}
	defer watcher.Stop()

	// Create persistence store
	store, err := storage.New("")
	if err != nil {
		return fmt.Errorf("storage: %w", err)
	}

	// Create chat manager
	chatMgr := chat.NewManager(server, kp, selfHost, store)
	defer chatMgr.Stop()

	// Wire message callbacks to TUI
	chatMgr.OnMessage(func(chatKey string, msg chat.Message) {
		if program != nil {
			program.Send(tui.IncomingMsg{ChatKey: chatKey, Message: msg})
		}
	})

	chatMgr.OnGroupInvite(func(invite *protocol.GroupInvite, from string) {
		if program != nil {
			program.Send(tui.GroupInviteMsg{Invite: invite, From: from})
		}
	})

	chatMgr.OnPeerConnect(func(hostname string) {
		if program != nil {
			program.Send(tui.PeerConnectMsg{Hostname: hostname})
		}
	})

	chatMgr.OnTyping(func(chatKey string, isTyping bool) {
		if program != nil {
			program.Send(tui.TypingMsg{ChatKey: chatKey, IsTyping: isTyping})
		}
	})

	chatMgr.OnReaction(func(chatKey string, msgID string) {
		if program != nil {
			program.Send(tui.ReactionUpdatedMsg{ChatKey: chatKey})
		}
	})

	chatMgr.OnStatus(func(hostname string, state string) {
		if program != nil {
			program.Send(tui.StatusUpdatedMsg{Hostname: hostname, State: state})
		}
	})

	// Create and run TUI
	model := tui.NewModel(chatMgr, watcher)
	program = tea.NewProgram(model, tea.WithAltScreen())

	if _, err := program.Run(); err != nil {
		return fmt.Errorf("tui: %w", err)
	}

	return nil
}
