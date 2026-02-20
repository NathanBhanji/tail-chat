package tui

import (
	"bytes"
	"image"
	"image/color"
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"

	"github.com/NathanBhanji/tail-chat/internal/chat"
	"github.com/NathanBhanji/tail-chat/internal/crypto"
	"github.com/NathanBhanji/tail-chat/internal/discovery"
	"github.com/NathanBhanji/tail-chat/internal/giphy"
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

// --- GIF Picker Snapshot Tests ---

func TestViewGifPickerLoading(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	m.width = 100
	m.height = 30
	m.view = ViewGifPicker
	m.focusPane = PaneChat
	m.activeChatKey = "alice"
	m.gifPicker = gifPicker{
		query:   "cats",
		loading: true,
		thumbs:  make(map[int]image.Image),
	}
	m.input.Placeholder = "Search GIFs..."
	m.input.Focus()

	out := m.View()

	if !strings.Contains(out, "GIF Search") {
		t.Error("expected 'GIF Search' header")
	}
	if !strings.Contains(out, "cats") {
		t.Error("expected query 'cats' in header")
	}
	if !strings.Contains(out, "Searching Giphy") {
		t.Error("expected loading indicator")
	}
	// Sidebar should still be visible
	if !strings.Contains(out, "alice") {
		t.Error("expected sidebar peer 'alice' visible during GIF picker")
	}
}

func TestViewGifPickerError(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	m.width = 100
	m.height = 30
	m.view = ViewGifPicker
	m.focusPane = PaneChat
	m.activeChatKey = "alice"
	m.gifPicker = gifPicker{
		query:  "test",
		err:    "giphy: status 429",
		thumbs: make(map[int]image.Image),
	}

	out := m.View()

	if !strings.Contains(out, "GIF Search") {
		t.Error("expected 'GIF Search' header")
	}
	if !strings.Contains(out, "429") {
		t.Error("expected error message in output")
	}
}

func TestViewGifPickerEmptyResults(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	m.width = 100
	m.height = 30
	m.view = ViewGifPicker
	m.focusPane = PaneChat
	m.activeChatKey = "alice"
	m.gifPicker = gifPicker{
		query:   "zzzzzzzzzznonexistent",
		results: []giphy.GIF{},
		thumbs:  make(map[int]image.Image),
	}

	out := m.View()

	if !strings.Contains(out, "No results") {
		t.Error("expected 'No results' message")
	}
}

func TestViewGifPickerWithResults(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	m.width = 100
	m.height = 40
	m.view = ViewGifPicker
	m.focusPane = PaneChat
	m.activeChatKey = "alice"
	m.gifPicker = gifPicker{
		query: "cats",
		results: []giphy.GIF{
			{Title: "funny cat", Images: giphy.Images{FixedHeight: giphy.ImageData{URL: "http://a.gif"}}},
			{Title: "cute kitten", Images: giphy.Images{FixedHeight: giphy.ImageData{URL: "http://b.gif"}}},
			{Title: "cat meme", Images: giphy.Images{FixedHeight: giphy.ImageData{URL: "http://c.gif"}}},
		},
		thumbs: make(map[int]image.Image),
		cursor: 0,
	}
	m.input.Placeholder = "Search GIFs..."
	m.input.Focus()

	out := m.View()

	if !strings.Contains(out, "GIF Search: cats") {
		t.Error("expected header with query")
	}
	// Titles should appear (at least the loading/labels)
	if !strings.Contains(out, "funny cat") {
		t.Error("expected first GIF title")
	}
	if !strings.Contains(out, "cute kitten") {
		t.Error("expected second GIF title")
	}
	if !strings.Contains(out, "cat meme") {
		t.Error("expected third GIF title")
	}
	// Help text
	if !strings.Contains(out, "navigate") {
		t.Error("expected navigation help text")
	}
	if !strings.Contains(out, "esc") {
		t.Error("expected esc help text")
	}
}

func TestViewGifPickerWithThumbnails(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	m.width = 100
	m.height = 40
	m.view = ViewGifPicker
	m.focusPane = PaneChat
	m.activeChatKey = "alice"

	// Create picker with loaded thumbnails
	thumbs := make(map[int]image.Image)
	thumbs[0] = testImage(50, 40, color.RGBA{R: 255, A: 255})
	thumbs[1] = testImage(50, 40, color.RGBA{G: 255, A: 255})

	m.gifPicker = gifPicker{
		query: "colors",
		results: []giphy.GIF{
			{Title: "red", Images: giphy.Images{FixedHeight: giphy.ImageData{URL: "http://red.gif"}}},
			{Title: "green", Images: giphy.Images{FixedHeight: giphy.ImageData{URL: "http://green.gif"}}},
		},
		thumbs: thumbs,
		cursor: 0,
	}

	out := m.View()

	// Should contain half-block characters from rendered thumbnails
	if !strings.Contains(out, "\u2580") {
		t.Error("expected half-block characters from rendered thumbnails")
	}
	if !strings.Contains(out, "red") {
		t.Error("expected 'red' title")
	}
	if !strings.Contains(out, "green") {
		t.Error("expected 'green' title")
	}
}

func TestViewGifPickerDefaultTitle(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	m.width = 100
	m.height = 40
	m.view = ViewGifPicker
	m.focusPane = PaneChat
	m.activeChatKey = "alice"
	m.gifPicker = gifPicker{
		query: "test",
		results: []giphy.GIF{
			{Title: "", Images: giphy.Images{}}, // empty title
		},
		thumbs: make(map[int]image.Image),
		cursor: 0,
	}

	out := m.View()

	// Should show default title "GIF 1" when title is empty
	if !strings.Contains(out, "GIF 1") {
		t.Error("expected default 'GIF 1' title for unnamed GIF")
	}
}

func TestViewGifPickerCursorHighlight(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	m.width = 100
	m.height = 40
	m.view = ViewGifPicker
	m.focusPane = PaneChat
	m.activeChatKey = "alice"

	// Test that cursor position affects rendering through key handling
	m.gifPicker = gifPicker{
		query:   "test",
		results: make([]giphy.GIF, 6),
		thumbs:  make(map[int]image.Image),
		cursor:  0,
	}

	// Move cursor right via key handling
	updated, _ := m.handleGifPickerKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	um := updated.(Model)
	if um.gifPicker.cursor != 1 {
		t.Errorf("cursor after right: %d, want 1", um.gifPicker.cursor)
	}

	// Move cursor down
	m.gifPicker.cursor = 0
	updated, _ = m.handleGifPickerKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	um = updated.(Model)
	if um.gifPicker.cursor != gifPickerGridCols {
		t.Errorf("cursor after down: %d, want %d", um.gifPicker.cursor, gifPickerGridCols)
	}
}

func TestViewGifPickerAtVariousWidths(t *testing.T) {
	widths := []int{40, 60, 80, 100, 150}

	for _, w := range widths {
		t.Run(strings.Repeat("w", w/10), func(t *testing.T) {
			m, cleanup := setupTestModel(t)
			defer cleanup()

			m.width = w
			m.height = 30
			m.view = ViewGifPicker
			m.activeChatKey = "alice"
			m.gifPicker = gifPicker{
				query: "test",
				results: []giphy.GIF{
					{Title: "a", Images: giphy.Images{}},
					{Title: "b", Images: giphy.Images{}},
				},
				thumbs: make(map[int]image.Image),
			}

			out := m.View()
			if len(out) == 0 {
				t.Errorf("width=%d: empty view output", w)
			}
			if !strings.Contains(out, "GIF Search") {
				t.Errorf("width=%d: missing header", w)
			}
		})
	}
}

// --- GIF Picker Key Handling Tests ---

func TestGifPickerKeys_EscReturnsToChat(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	m.width = 100
	m.height = 30
	m.view = ViewGifPicker
	m.focusPane = PaneChat
	m.activeChatKey = "alice"
	m.gifPicker = gifPicker{
		query:   "cats",
		results: []giphy.GIF{{Title: "cat"}},
		thumbs:  make(map[int]image.Image),
	}

	updated, _ := m.handleGifPickerKeys(tea.KeyMsg{Type: tea.KeyEscape})
	um := updated.(Model)

	if um.view != ViewChat {
		t.Errorf("expected ViewChat after esc, got %d", um.view)
	}
	if um.focusPane != PaneChat {
		t.Errorf("expected PaneChat after esc, got %d", um.focusPane)
	}
}

func TestGifPickerKeys_EscReturnsToEmpty(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	m.width = 100
	m.height = 30
	m.view = ViewGifPicker
	m.focusPane = PaneChat
	m.activeChatKey = "" // no active chat
	m.gifPicker = gifPicker{thumbs: make(map[int]image.Image)}

	updated, _ := m.handleGifPickerKeys(tea.KeyMsg{Type: tea.KeyEscape})
	um := updated.(Model)

	if um.view != ViewEmpty {
		t.Errorf("expected ViewEmpty after esc with no active chat, got %d", um.view)
	}
	if um.focusPane != PaneSidebar {
		t.Errorf("expected PaneSidebar, got %d", um.focusPane)
	}
}

func TestGifPickerKeys_ArrowNavigation(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	m.width = 100
	m.height = 30
	m.view = ViewGifPicker
	m.focusPane = PaneChat
	m.gifPicker = gifPicker{
		results: make([]giphy.GIF, 9), // 3x3 grid
		thumbs:  make(map[int]image.Image),
		cursor:  0,
	}

	// Right
	updated, _ := m.handleGifPickerKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	um := updated.(Model)
	if um.gifPicker.cursor != 1 {
		t.Errorf("right: cursor = %d, want 1", um.gifPicker.cursor)
	}

	// Down (should jump by gridCols = 3)
	m.gifPicker.cursor = 0
	updated, _ = m.handleGifPickerKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	um = updated.(Model)
	if um.gifPicker.cursor != 3 {
		t.Errorf("down: cursor = %d, want 3", um.gifPicker.cursor)
	}

	// Up from row 2
	m.gifPicker.cursor = 4
	updated, _ = m.handleGifPickerKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	um = updated.(Model)
	if um.gifPicker.cursor != 1 {
		t.Errorf("up: cursor = %d, want 1", um.gifPicker.cursor)
	}

	// Left
	m.gifPicker.cursor = 2
	updated, _ = m.handleGifPickerKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	um = updated.(Model)
	if um.gifPicker.cursor != 1 {
		t.Errorf("left: cursor = %d, want 1", um.gifPicker.cursor)
	}
}

func TestGifPickerKeys_CursorBounds(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	m.view = ViewGifPicker
	m.gifPicker = gifPicker{
		results: make([]giphy.GIF, 3),
		thumbs:  make(map[int]image.Image),
		cursor:  0,
	}

	// Left at 0 should stay at 0
	updated, _ := m.handleGifPickerKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	um := updated.(Model)
	if um.gifPicker.cursor != 0 {
		t.Errorf("left at 0: cursor = %d, want 0", um.gifPicker.cursor)
	}

	// Right at last should stay at last
	m.gifPicker.cursor = 2
	updated, _ = m.handleGifPickerKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	um = updated.(Model)
	if um.gifPicker.cursor != 2 {
		t.Errorf("right at end: cursor = %d, want 2", um.gifPicker.cursor)
	}

	// Up at row 0 should stay
	m.gifPicker.cursor = 1
	updated, _ = m.handleGifPickerKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	um = updated.(Model)
	if um.gifPicker.cursor != 1 {
		t.Errorf("up at row 0: cursor = %d, want 1", um.gifPicker.cursor)
	}

	// Down when would exceed results should stay
	m.gifPicker.cursor = 1
	updated, _ = m.handleGifPickerKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	um = updated.(Model)
	if um.gifPicker.cursor != 1 {
		t.Errorf("down beyond end: cursor = %d, want 1", um.gifPicker.cursor)
	}
}

func TestGifPickerKeys_EnterSendsGIF(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	m.view = ViewGifPicker
	m.focusPane = PaneChat
	m.activeChatKey = "alice"
	m.gifPicker = gifPicker{
		query: "test",
		results: []giphy.GIF{
			{
				Title: "selected",
				Images: giphy.Images{
					FixedHeight: giphy.ImageData{URL: "https://media.giphy.com/selected.gif"},
				},
			},
		},
		thumbs: make(map[int]image.Image),
		cursor: 0,
	}
	m.input.SetValue("") // empty input = send selected

	updated, cmd := m.handleGifPickerKeys(tea.KeyMsg{Type: tea.KeyEnter})
	um := updated.(Model)

	// Should return to chat view
	if um.view != ViewChat {
		t.Errorf("expected ViewChat after sending GIF, got %d", um.view)
	}

	// Should have a command (async send)
	if cmd == nil {
		t.Fatal("expected non-nil command for GIF send")
	}

	// Execute the command to verify it produces MessageSentMsg
	msg := cmd()
	if _, ok := msg.(MessageSentMsg); !ok {
		t.Errorf("expected MessageSentMsg, got %T", msg)
	}
}

func TestGifPickerKeys_EnterWithTextSearches(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	m.view = ViewGifPicker
	m.focusPane = PaneChat
	m.activeChatKey = "alice"
	m.gifPicker = gifPicker{
		query:   "old query",
		results: []giphy.GIF{{Title: "old"}},
		thumbs:  make(map[int]image.Image),
		cursor:  0,
	}
	m.input.SetValue("new search")

	updated, cmd := m.handleGifPickerKeys(tea.KeyMsg{Type: tea.KeyEnter})
	um := updated.(Model)

	// Should stay in picker view with new query
	if um.view != ViewGifPicker {
		t.Errorf("expected ViewGifPicker for new search, got %d", um.view)
	}
	if um.gifPicker.query != "new search" {
		t.Errorf("expected query 'new search', got %q", um.gifPicker.query)
	}
	if !um.gifPicker.loading {
		t.Error("expected loading=true for new search")
	}
	if um.gifPicker.cursor != 0 {
		t.Error("expected cursor reset to 0 for new search")
	}
	if cmd == nil {
		t.Fatal("expected non-nil command for search")
	}
}

func TestGifPickerKeys_EnterEmptyResultsNoOp(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	m.view = ViewGifPicker
	m.gifPicker = gifPicker{
		results: nil, // no results
		thumbs:  make(map[int]image.Image),
	}
	m.input.SetValue("")

	updated, cmd := m.handleGifPickerKeys(tea.KeyMsg{Type: tea.KeyEnter})
	um := updated.(Model)

	// Should stay in picker (nothing to send)
	if um.view != ViewGifPicker {
		t.Errorf("expected ViewGifPicker, got %d", um.view)
	}
	if cmd != nil {
		t.Error("expected nil command when nothing to send")
	}
}

// --- GIF Command Integration Tests ---

func TestChatKeys_GifCommand(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	m.width = 100
	m.height = 30
	m.view = ViewChat
	m.focusPane = PaneChat
	m.activeChatKey = "alice"
	m.input.SetValue("/gif cats")
	m.input.Focus()

	updated, cmd := m.handleChatKeys(tea.KeyMsg{Type: tea.KeyEnter})
	um := updated.(Model)

	if um.view != ViewGifPicker {
		t.Errorf("expected ViewGifPicker, got %d", um.view)
	}
	if um.gifPicker.query != "cats" {
		t.Errorf("expected query 'cats', got %q", um.gifPicker.query)
	}
	if !um.gifPicker.loading {
		t.Error("expected loading=true")
	}
	if cmd == nil {
		t.Fatal("expected search command")
	}
}

func TestChatKeys_GifCommandNoQuery(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	m.view = ViewChat
	m.focusPane = PaneChat
	m.activeChatKey = "alice"
	m.input.SetValue("/gif")

	updated, cmd := m.handleChatKeys(tea.KeyMsg{Type: tea.KeyEnter})
	um := updated.(Model)

	if um.view != ViewGifPicker {
		t.Errorf("expected ViewGifPicker, got %d", um.view)
	}
	if um.gifPicker.query != "trending" {
		t.Errorf("expected 'trending' default query, got %q", um.gifPicker.query)
	}
	if cmd == nil {
		t.Fatal("expected search command")
	}
}

func TestChatKeys_GifCommandWithSpaces(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	m.view = ViewChat
	m.focusPane = PaneChat
	m.activeChatKey = "alice"
	m.input.SetValue("/gif funny dancing cat")

	updated, _ := m.handleChatKeys(tea.KeyMsg{Type: tea.KeyEnter})
	um := updated.(Model)

	if um.gifPicker.query != "funny dancing cat" {
		t.Errorf("expected multi-word query, got %q", um.gifPicker.query)
	}
}

// --- Update() Message Handling Tests ---

func TestUpdate_GifSearchResultMsg(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	m.width = 100
	m.height = 30
	m.view = ViewGifPicker
	m.gifPicker = gifPicker{
		query:   "cats",
		loading: true,
		thumbs:  make(map[int]image.Image),
	}

	results := []giphy.GIF{
		{Title: "cat1", Images: giphy.Images{FixedHeightStill: giphy.ImageData{URL: "http://1.gif"}}},
		{Title: "cat2", Images: giphy.Images{FixedHeightStill: giphy.ImageData{URL: "http://2.gif"}}},
	}

	updated, cmd := m.Update(GifSearchResultMsg{Results: results})
	um := updated.(Model)

	if um.gifPicker.loading {
		t.Error("expected loading=false after results")
	}
	if len(um.gifPicker.results) != 2 {
		t.Errorf("expected 2 results, got %d", len(um.gifPicker.results))
	}
	if um.gifPicker.cursor != 0 {
		t.Error("expected cursor reset to 0")
	}
	if cmd == nil {
		t.Fatal("expected thumbnail loading command")
	}
}

func TestUpdate_GifSearchResultMsg_Error(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	m.view = ViewGifPicker
	m.gifPicker = gifPicker{
		query:   "cats",
		loading: true,
		thumbs:  make(map[int]image.Image),
	}

	updated, _ := m.Update(GifSearchResultMsg{Err: image.ErrFormat})
	um := updated.(Model)

	if um.gifPicker.loading {
		t.Error("expected loading=false after error")
	}
	if um.gifPicker.err == "" {
		t.Error("expected error message to be set")
	}
}

func TestUpdate_GifThumbLoadedMsg(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	m.view = ViewGifPicker
	m.gifPicker = gifPicker{
		query:   "cats",
		results: make([]giphy.GIF, 3),
		thumbs:  make(map[int]image.Image),
	}

	img := testImage(50, 40, color.RGBA{R: 200, A: 255})
	updated, _ := m.Update(GifThumbLoadedMsg{Index: 1, Img: img})
	um := updated.(Model)

	if _, ok := um.gifPicker.thumbs[1]; !ok {
		t.Error("expected thumbnail stored at index 1")
	}
}

func TestUpdate_GifThumbLoadedMsg_Error(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	m.view = ViewGifPicker
	m.gifPicker = gifPicker{
		thumbs: make(map[int]image.Image),
	}

	updated, _ := m.Update(GifThumbLoadedMsg{Index: 0, Err: image.ErrFormat})
	um := updated.(Model)

	if _, ok := um.gifPicker.thumbs[0]; ok {
		t.Error("should not store thumbnail on error")
	}
}

func TestUpdate_ImageCachedMsg(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	// ImageCachedMsg should not panic or cause issues
	updated, _ := m.Update(ImageCachedMsg{URL: "http://example.com/test.gif"})
	if updated == nil {
		t.Error("expected non-nil model")
	}
}

// --- Inline Image in Chat View Tests ---

func TestViewChatWithImageURL(t *testing.T) {
	clearImageCache()
	defer clearImageCache()

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
			Content:   "https://media.giphy.com/media/abc123/200.gif",
			Timestamp: time.Now(),
		},
	}

	// Without cached image, should show [GIF] label
	out := m.View()
	if !strings.Contains(out, "[GIF]") {
		t.Error("expected [GIF] label for uncached image URL")
	}
}

func TestViewChatWithCachedImage(t *testing.T) {
	clearImageCache()
	defer clearImageCache()

	// Force non-Kitty mode for predictable output
	kittyDetected = nil
	origTerm := os.Getenv("TERM")
	origProg := os.Getenv("TERM_PROGRAM")
	os.Setenv("TERM", "xterm-256color")
	os.Setenv("TERM_PROGRAM", "")
	defer func() {
		os.Setenv("TERM", origTerm)
		os.Setenv("TERM_PROGRAM", origProg)
		kittyDetected = nil
	}()

	url := "https://media.giphy.com/media/abc123/200.gif"
	setCachedImage(url, testImage(80, 60, color.RGBA{R: 200, G: 100, B: 50, A: 255}), nil)

	m, cleanup := setupTestModel(t)
	defer cleanup()

	m.width = 100
	m.height = 40
	m.view = ViewChat
	m.focusPane = PaneChat
	m.activeChatKey = "alice"
	m.messages = []chat.Message{
		{
			ID:        "1",
			Sender:    "alice",
			Content:   url,
			Timestamp: time.Now(),
		},
	}

	out := m.View()

	// Should contain [GIF] label
	if !strings.Contains(out, "[GIF]") {
		t.Error("expected [GIF] label")
	}

	// Should contain half-block characters from inline rendering
	if !strings.Contains(out, "\u2580") {
		t.Error("expected half-block inline image rendering")
	}
}

func TestViewChatHelp_IncludesGif(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	m.width = 100
	m.height = 30
	m.view = ViewChat
	m.focusPane = PaneChat
	m.activeChatKey = "alice"

	out := m.View()

	if !strings.Contains(out, "/gif") {
		t.Error("expected /gif in help text")
	}
}

// --- Interactive GIF Picker Tests ---

func TestInteractiveGifPickerOpen(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	m.view = ViewChat
	m.activeChatKey = "alice"
	m.focusPane = PaneChat
	m.input.Focus()

	tm := newTestProgram(t, m)
	waitForText(t, tm, "alice")

	// Type /gif cats and press enter
	tm.Type("/gif cats")
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	// Should show GIF Search header
	waitForText(t, tm, "GIF Search")

	tm.Send(tea.QuitMsg{})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestInteractiveGifPickerEsc(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	m.view = ViewGifPicker
	m.activeChatKey = "alice"
	m.focusPane = PaneChat
	m.gifPicker = gifPicker{
		query:  "test",
		thumbs: make(map[int]image.Image),
	}
	m.input.Placeholder = "Search GIFs..."
	m.input.Focus()

	tm := newTestProgram(t, m)
	waitForText(t, tm, "GIF Search")

	// Esc should return to chat
	tm.Send(tea.KeyMsg{Type: tea.KeyEscape})
	time.Sleep(200 * time.Millisecond)

	tm.Send(tea.QuitMsg{})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}
