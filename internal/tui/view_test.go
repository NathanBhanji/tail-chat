package tui

import (
	"bytes"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"

	"github.com/NathanBhanji/tail-chat/internal/chat"
	"github.com/NathanBhanji/tail-chat/internal/crypto"
	"github.com/NathanBhanji/tail-chat/internal/discovery"
	tcnet "github.com/NathanBhanji/tail-chat/internal/net"
)

// testPeers returns a realistic set of peers for testing.
func testPeers() []discovery.Peer {
	return []discovery.Peer{
		{Hostname: "alice", TailscaleIP: "100.64.0.1", Online: true, RunningTailchat: true},
		{Hostname: "bob", TailscaleIP: "100.64.0.2", Online: true, RunningTailchat: false},
		{Hostname: "charlie", TailscaleIP: "100.64.0.3", Online: false},
	}
}

// setupTestModel creates a Model suitable for teatest with a real Manager
// and a test Watcher (no Tailscale required).
func setupTestModel(t *testing.T) (Model, func()) {
	t.Helper()

	kp, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	srv, err := tcnet.NewServer("127.0.0.1:0", kp, "me", nil)
	if err != nil {
		t.Fatalf("server: %v", err)
	}
	srv.Start()

	mgr := chat.NewManager(srv, kp, "me", nil, nil)
	watcher := discovery.NewTestWatcher("me", testPeers())

	m := NewModel(mgr, watcher)

	cleanup := func() {
		mgr.Stop()
		srv.Stop()
	}

	return m, cleanup
}

// newTestProgram creates a teatest.TestModel with a fixed terminal size.
func newTestProgram(t *testing.T, m Model) *teatest.TestModel {
	return teatest.NewTestModel(t, m, teatest.WithInitialTermSize(100, 30))
}

// waitForText waits for specific text to appear in the program output.
func waitForText(t *testing.T, tm *teatest.TestModel, text string) {
	t.Helper()
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte(text))
	}, teatest.WithDuration(3*time.Second), teatest.WithCheckInterval(100*time.Millisecond))
}

// --- Snapshot Tests (View() output without full teatest loop) ---

func TestViewEmpty(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	m.width = 100
	m.height = 30

	out := m.View()

	// Should show sidebar with peers
	if !strings.Contains(out, "tailchat") {
		t.Error("expected 'tailchat' title in sidebar")
	}
	if !strings.Contains(out, "alice") {
		t.Error("expected 'alice' in peer list")
	}
	if !strings.Contains(out, "bob") {
		t.Error("expected 'bob' in peer list")
	}
	if !strings.Contains(out, "charlie") {
		t.Error("expected 'charlie' in peer list")
	}

	// Should show welcome/empty pane
	if !strings.Contains(out, "Select a peer") {
		t.Error("expected welcome message in empty pane")
	}

	// Should show key hints
	if !strings.Contains(out, "navigate") {
		t.Error("expected navigation help text")
	}
}

func TestViewChat(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	m.width = 100
	m.height = 30
	m.view = ViewChat
	m.focusPane = PaneChat
	m.activeChatKey = "alice"
	m.messages = []chat.Message{
		{
			ID:        "1",
			Sender:    "alice",
			Content:   "hey there!",
			Timestamp: time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
		},
		{
			ID:        "2",
			Sender:    "me",
			Content:   "hi alice!",
			Timestamp: time.Date(2025, 1, 15, 10, 31, 0, 0, time.UTC),
			IsOwn:     true,
			State:     chat.StateDelivered,
		},
	}
	m.input.Focus()

	out := m.View()

	// Sidebar should still be visible
	if !strings.Contains(out, "bob") {
		t.Error("expected sidebar with peer 'bob' visible alongside chat")
	}

	// Chat pane should show messages
	if !strings.Contains(out, "hey there!") {
		t.Error("expected alice's message content")
	}
	if !strings.Contains(out, "hi alice!") {
		t.Error("expected own message content")
	}

	// Should show the chat header with peer name
	if !strings.Contains(out, "alice") {
		t.Error("expected chat header with 'alice'")
	}

	// Should show delivery tick for own message
	if !strings.Contains(out, "\u2713") {
		t.Error("expected delivery tick for delivered message")
	}
}

func TestViewChatWithReactions(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	m.width = 100
	m.height = 30
	m.view = ViewChat
	m.focusPane = PaneChat
	m.activeChatKey = "alice"
	m.messages = []chat.Message{
		{
			ID:        "1",
			Sender:    "alice",
			Content:   "check this out",
			Timestamp: time.Now(),
			Reactions: []chat.Reaction{
				{Emoji: "\U0001f44d", Sender: "me"},
				{Emoji: "\U0001f525", Sender: "me"},
			},
		},
	}

	out := m.View()

	if !strings.Contains(out, "\U0001f44d") {
		t.Error("expected thumbsup reaction")
	}
	if !strings.Contains(out, "\U0001f525") {
		t.Error("expected fire reaction")
	}
}

func TestViewFileTransfer(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	m.width = 100
	m.height = 30
	m.view = ViewChat
	m.activeChatKey = "alice"
	m.messages = []chat.Message{
		{
			ID:        "f1",
			Sender:    "me",
			IsOwn:     true,
			Timestamp: time.Now(),
			FileInfo: &chat.FileInfo{
				Filename: "report.pdf",
				Size:     1048576, // 1 MB
				State:    chat.FileSent,
			},
		},
		{
			ID:        "f2",
			Sender:    "alice",
			Timestamp: time.Now(),
			FileInfo: &chat.FileInfo{
				Filename: "photo.jpg",
				Size:     524288,
				State:    chat.FileReceived,
				Path:     "~/Downloads/tailchat/photo.jpg",
			},
		},
	}

	out := m.View()

	if !strings.Contains(out, "report.pdf") {
		t.Error("expected sent filename")
	}
	if !strings.Contains(out, "1.0 MB") {
		t.Error("expected formatted file size")
	}
	if !strings.Contains(out, "photo.jpg") {
		t.Error("expected received filename")
	}
	if !strings.Contains(out, "received") {
		t.Error("expected 'received' status")
	}
}

func TestViewSidebarPeerStatus(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	m.width = 100
	m.height = 30

	out := m.View()

	// Verify all peers are listed
	if !strings.Contains(out, "alice") {
		t.Error("expected alice in sidebar")
	}
	if !strings.Contains(out, "bob") {
		t.Error("expected bob in sidebar")
	}
	if !strings.Contains(out, "charlie") {
		t.Error("expected charlie in sidebar")
	}

	// Verify status dots are present (●/○ unicode chars)
	if !strings.Contains(out, "\u25cf") && !strings.Contains(out, "\u25cb") {
		t.Error("expected status dot indicators")
	}
}

func TestViewGroupCreate(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	m.width = 100
	m.height = 30
	m.view = ViewGroupCreate
	m.focusPane = PaneChat
	m.groupName = ""
	m.input.Placeholder = "Group name..."
	m.input.Focus()

	out := m.View()

	if !strings.Contains(out, "New Group") {
		t.Error("expected 'New Group' header")
	}
	if !strings.Contains(out, "group name") {
		t.Error("expected group name prompt")
	}
}

func TestViewSearch(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	m.width = 100
	m.height = 30
	m.view = ViewSearch
	m.focusPane = PaneChat
	m.searchDone = false
	m.input.Placeholder = "Search messages..."
	m.input.Focus()

	out := m.View()

	if !strings.Contains(out, "Search") {
		t.Error("expected 'Search' header")
	}
	if !strings.Contains(out, "search query") {
		t.Error("expected search query prompt")
	}
}

func TestViewSearchResults(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	m.width = 100
	m.height = 30
	m.view = ViewSearch
	m.focusPane = PaneChat
	m.searchDone = true
	m.searchResults = []searchResult{
		{ChatKey: "alice", Message: chat.Message{Sender: "alice", Content: "hello world", Timestamp: time.Now()}},
		{ChatKey: "bob", Message: chat.Message{Sender: "bob", Content: "goodbye", Timestamp: time.Now()}},
	}

	out := m.View()

	if !strings.Contains(out, "2 matches") {
		t.Error("expected '2 matches' in results header")
	}
	if !strings.Contains(out, "hello world") {
		t.Error("expected search result content")
	}
}

func TestSidebarFocusStyles(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	m.width = 100
	m.height = 30

	// Sidebar focused
	m.focusPane = PaneSidebar
	out1 := m.View()

	// Chat focused
	m.focusPane = PaneChat
	m.view = ViewChat
	m.activeChatKey = "alice"
	out2 := m.View()

	// Both should render (no panic), and should differ due to focus styles
	if out1 == out2 {
		t.Error("expected different rendering when focus changes")
	}
}

func TestSidebarWidth(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	m.height = 30

	// Narrow terminal
	m.width = 60
	sw1 := m.sidebarWidth()
	if sw1 < 15 || sw1 > 35 {
		t.Errorf("narrow sidebar width %d out of expected range", sw1)
	}

	// Wide terminal
	m.width = 200
	sw2 := m.sidebarWidth()
	if sw2 > 35 {
		t.Errorf("wide sidebar width %d should be capped at 35", sw2)
	}

	// Very narrow terminal
	m.width = 30
	sw3 := m.sidebarWidth()
	if sw3 < 20 {
		t.Errorf("very narrow sidebar width %d should have minimum 20", sw3)
	}
}

func TestViewLoadingState(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	// Width 0 = not yet initialized
	m.width = 0
	out := m.View()
	if out != "Loading..." {
		t.Errorf("expected 'Loading...', got %q", out)
	}
}

func TestViewScrollIndicator(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	m.width = 100
	m.height = 30
	m.view = ViewChat
	m.activeChatKey = "alice"
	m.scrollOffset = 5

	var msgs []chat.Message
	for i := 0; i < 20; i++ {
		msgs = append(msgs, chat.Message{
			ID:        string(rune('a' + i)),
			Sender:    "alice",
			Content:   "message content",
			Timestamp: time.Now(),
		})
	}
	m.messages = msgs

	out := m.View()

	if !strings.Contains(out, "older messages") {
		t.Error("expected scroll indicator when scrollOffset > 0")
	}
}

func TestViewErrorDisplay(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	m.width = 100
	m.height = 30
	m.view = ViewChat
	m.activeChatKey = "alice"
	m.err = "connection lost"
	m.errExpiry = time.Now().Add(5 * time.Second)

	out := m.View()

	if !strings.Contains(out, "connection lost") {
		t.Error("expected error message in view")
	}
}

// --- Interactive Tests (teatest) ---

func TestInteractiveStartup(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	tm := newTestProgram(t, m)

	// Should show the sidebar and welcome pane after window size
	waitForText(t, tm, "tailchat")

	tm.Send(tea.QuitMsg{})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestInteractiveSidebarNavigation(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	tm := newTestProgram(t, m)
	waitForText(t, tm, "tailchat")

	// Move down to second peer (bob)
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	time.Sleep(100 * time.Millisecond)

	// Move down to third peer (charlie)
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	time.Sleep(100 * time.Millisecond)

	// Move back up
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	time.Sleep(100 * time.Millisecond)

	tm.Send(tea.QuitMsg{})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestInteractiveTabFocus(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	// Set up with an active chat so tab has something to switch to
	m.view = ViewChat
	m.activeChatKey = "alice"
	m.focusPane = PaneSidebar

	tm := newTestProgram(t, m)
	waitForText(t, tm, "alice")

	// Tab should switch to chat pane
	tm.Send(tea.KeyMsg{Type: tea.KeyTab})
	time.Sleep(200 * time.Millisecond)

	// Tab back to sidebar
	tm.Send(tea.KeyMsg{Type: tea.KeyTab})
	time.Sleep(200 * time.Millisecond)

	tm.Send(tea.QuitMsg{})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestInteractiveEscReturnsSidebar(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	m.view = ViewChat
	m.activeChatKey = "alice"
	m.focusPane = PaneChat
	m.input.Focus()

	tm := newTestProgram(t, m)
	waitForText(t, tm, "alice")

	// Esc should return focus to sidebar
	tm.Send(tea.KeyMsg{Type: tea.KeyEscape})
	time.Sleep(200 * time.Millisecond)

	tm.Send(tea.QuitMsg{})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestInteractiveSearchView(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	tm := newTestProgram(t, m)
	waitForText(t, tm, "tailchat")

	// / should open search
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})

	waitForText(t, tm, "Search")

	// Esc should close search
	tm.Send(tea.KeyMsg{Type: tea.KeyEscape})
	time.Sleep(200 * time.Millisecond)

	tm.Send(tea.QuitMsg{})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestInteractiveGroupCreate(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	tm := newTestProgram(t, m)
	waitForText(t, tm, "tailchat")

	// n should open group create
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})

	waitForText(t, tm, "New Group")

	// Esc should close it
	tm.Send(tea.KeyMsg{Type: tea.KeyEscape})
	time.Sleep(200 * time.Millisecond)

	tm.Send(tea.QuitMsg{})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestInteractiveQuit(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	tm := newTestProgram(t, m)
	waitForText(t, tm, "tailchat")

	// q should quit
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})

	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestInteractiveCtrlCQuit(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	tm := newTestProgram(t, m)
	waitForText(t, tm, "tailchat")

	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})

	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

// --- Layout Consistency Tests ---

func TestLayoutAtVariousWidths(t *testing.T) {
	widths := []int{40, 60, 80, 100, 120, 160, 200}

	for _, w := range widths {
		t.Run(strings.ReplaceAll(string(rune(w+'0')), "\x00", ""), func(t *testing.T) {
			m, cleanup := setupTestModel(t)
			defer cleanup()

			m.width = w
			m.height = 30
			m.view = ViewChat
			m.activeChatKey = "alice"
			m.messages = []chat.Message{
				{ID: "1", Sender: "alice", Content: "test message", Timestamp: time.Now()},
			}

			out := m.View()

			// Should not panic and should contain content
			if len(out) == 0 {
				t.Error("empty view output")
			}

			// Should contain both panes
			if !strings.Contains(out, "alice") {
				t.Errorf("width=%d: expected peer name in output", w)
			}
			if !strings.Contains(out, "test") {
				t.Errorf("width=%d: expected message content in output", w)
			}
		})
	}
}

func TestLayoutAtVariousHeights(t *testing.T) {
	heights := []int{10, 15, 20, 30, 40, 50}

	for _, h := range heights {
		t.Run(strings.ReplaceAll(string(rune(h+'0')), "\x00", ""), func(t *testing.T) {
			m, cleanup := setupTestModel(t)
			defer cleanup()

			m.width = 100
			m.height = h
			m.view = ViewChat
			m.activeChatKey = "alice"

			out := m.View()

			if len(out) == 0 {
				t.Errorf("height=%d: empty view output", h)
			}
		})
	}
}

// --- Visual Layout Verification ---

func TestLayoutChatRender(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	m.width = 100
	m.height = 30
	m.view = ViewChat
	m.focusPane = PaneChat
	m.activeChatKey = "alice"
	m.messages = []chat.Message{
		{ID: "1", Sender: "alice", Content: "hey, how's it going?", Timestamp: time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)},
		{ID: "2", Sender: "me", Content: "pretty good!", Timestamp: time.Date(2025, 1, 15, 10, 31, 0, 0, time.UTC), IsOwn: true, State: chat.StateDelivered},
		{ID: "3", Sender: "alice", Content: "nice!", Timestamp: time.Date(2025, 1, 15, 10, 32, 0, 0, time.UTC), Reactions: []chat.Reaction{{Emoji: "\U0001f44d", Sender: "me"}}},
	}
	m.input.Focus()

	out := m.View()

	// Verify the layout has no broken borders (border chars should not appear mid-line
	// without being part of a proper box)
	lines := strings.Split(out, "\n")
	for i, line := range lines {
		// Check no line exceeds terminal width (accounting for ANSI codes)
		visLen := visualWidth(line)
		if visLen > m.width+2 { // +2 for minor rounding with double-wide chars
			t.Errorf("line %d visual width %d exceeds terminal width %d: %q", i, visLen, m.width, line)
		}
	}

	// Verify > prompt renders (no more box border)
	promptFound := false
	for _, line := range lines {
		if strings.Contains(line, ">") {
			promptFound = true
			break
		}
	}
	if !promptFound {
		t.Error("> prompt not found in rendered output")
	}

	// Verify header divider renders
	dividerFound := false
	for _, line := range lines {
		if strings.Contains(line, "\u2500\u2500\u2500") {
			dividerFound = true
			break
		}
	}
	if !dividerFound {
		t.Error("header divider ─── not found in rendered output")
	}
}

// visualWidth approximates the visual width of a string by stripping ANSI codes.
func visualWidth(s string) int {
	inEsc := false
	w := 0
	for _, r := range s {
		if r == '\x1b' {
			inEsc = true
			continue
		}
		if inEsc {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '~' {
				inEsc = false
			}
			continue
		}
		w++
	}
	return w
}
