package chat_test

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/NathanBhanji/tail-chat/internal/chat"
	"github.com/NathanBhanji/tail-chat/internal/crypto"
	tcnet "github.com/NathanBhanji/tail-chat/internal/net"
	"github.com/NathanBhanji/tail-chat/internal/storage"
)

// setupPair creates two connected chat managers for testing.
func setupPair(t *testing.T) (alice *chat.Manager, bob *chat.Manager, cleanup func()) {
	t.Helper()

	kpA, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate alice key: %v", err)
	}
	kpB, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate bob key: %v", err)
	}

	srvA, err := tcnet.NewServer("127.0.0.1:0", kpA, "alice", nil)
	if err != nil {
		t.Fatalf("server alice: %v", err)
	}
	srvA.Start()

	srvB, err := tcnet.NewServer("127.0.0.1:0", kpB, "bob", nil)
	if err != nil {
		srvA.Stop()
		t.Fatalf("server bob: %v", err)
	}
	srvB.Start()

	alice = chat.NewManager(srvA, kpA, "alice", nil, nil)
	bob = chat.NewManager(srvB, kpB, "bob", nil, nil)

	// Connect alice -> bob
	conn, err := tcnet.Connect(srvB.Addr(), kpA, "alice", nil)
	if err != nil {
		srvA.Stop()
		srvB.Stop()
		t.Fatalf("connect alice->bob: %v", err)
	}
	srvA.AddConnection(conn)

	// Wait for bob's server to accept and register the connection
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if bob.IsConnected("alice") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !bob.IsConnected("alice") {
		srvA.Stop()
		srvB.Stop()
		t.Fatalf("bob never saw alice's connection")
	}

	cleanup = func() {
		alice.Stop()
		bob.Stop()
		srvA.Stop()
		srvB.Stop()
	}
	return
}

func TestSendAndReceive(t *testing.T) {
	alice, bob, cleanup := setupPair(t)
	defer cleanup()

	// Track received messages
	var mu sync.Mutex
	bobGot := make([]chat.Message, 0)
	bob.OnMessage(func(chatKey string, msg chat.Message) {
		mu.Lock()
		bobGot = append(bobGot, msg)
		mu.Unlock()
	})

	aliceGot := make([]chat.Message, 0)
	alice.OnMessage(func(chatKey string, msg chat.Message) {
		mu.Lock()
		aliceGot = append(aliceGot, msg)
		mu.Unlock()
	})

	// Alice sends to bob
	if err := alice.SendMessage("bob", "hello bob"); err != nil {
		t.Fatalf("alice send: %v", err)
	}

	// Wait for bob to receive
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := len(bobGot)
		mu.Unlock()
		if n > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	mu.Lock()
	if len(bobGot) == 0 {
		t.Fatal("bob never received alice's message")
	}
	if bobGot[0].Content != "hello bob" {
		t.Fatalf("bob got wrong content: %q", bobGot[0].Content)
	}
	if bobGot[0].Sender != "alice" {
		t.Fatalf("bob got wrong sender: %q", bobGot[0].Sender)
	}
	mu.Unlock()

	// Verify alice sees her own message in history
	aliceMsgs := alice.GetMessages("bob")
	if len(aliceMsgs) != 1 {
		t.Fatalf("alice history: expected 1, got %d", len(aliceMsgs))
	}
	if !aliceMsgs[0].IsOwn {
		t.Fatal("alice's own message not marked as own")
	}

	// Verify bob sees the message in history
	bobMsgs := bob.GetMessages("alice")
	if len(bobMsgs) != 1 {
		t.Fatalf("bob history: expected 1, got %d", len(bobMsgs))
	}
	if bobMsgs[0].IsOwn {
		t.Fatal("bob should not see alice's message as own")
	}
}

func TestBidirectionalChat(t *testing.T) {
	alice, bob, cleanup := setupPair(t)
	defer cleanup()

	var mu sync.Mutex
	bobGot := make([]chat.Message, 0)
	bob.OnMessage(func(chatKey string, msg chat.Message) {
		mu.Lock()
		bobGot = append(bobGot, msg)
		mu.Unlock()
	})

	aliceGot := make([]chat.Message, 0)
	alice.OnMessage(func(chatKey string, msg chat.Message) {
		mu.Lock()
		aliceGot = append(aliceGot, msg)
		mu.Unlock()
	})

	// Alice sends to bob
	if err := alice.SendMessage("bob", "hi bob"); err != nil {
		t.Fatalf("alice send: %v", err)
	}

	// Wait for delivery
	waitFor(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(bobGot) >= 1
	}, "bob to receive alice's message")

	// Bob replies to alice
	if err := bob.SendMessage("alice", "hi alice"); err != nil {
		t.Fatalf("bob send: %v", err)
	}

	// Wait for delivery
	waitFor(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		// aliceGot includes alice's own message callback + bob's reply
		for _, m := range aliceGot {
			if m.Sender == "bob" {
				return true
			}
		}
		return false
	}, "alice to receive bob's reply")

	// Verify both histories
	aliceMsgs := alice.GetMessages("bob")
	if len(aliceMsgs) < 2 {
		t.Fatalf("alice history: expected >= 2, got %d", len(aliceMsgs))
	}

	bobMsgs := bob.GetMessages("alice")
	if len(bobMsgs) < 2 {
		t.Fatalf("bob history: expected >= 2, got %d", len(bobMsgs))
	}
}

func TestConcurrentSends(t *testing.T) {
	alice, bob, cleanup := setupPair(t)
	defer cleanup()

	var mu sync.Mutex
	bobGot := make([]chat.Message, 0)
	bob.OnMessage(func(chatKey string, msg chat.Message) {
		if msg.Sender == "alice" {
			mu.Lock()
			bobGot = append(bobGot, msg)
			mu.Unlock()
		}
	})

	// Send 20 messages concurrently
	const N = 20
	var wg sync.WaitGroup
	var sendErrors []error
	var errMu sync.Mutex

	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := alice.SendMessage("bob", fmt.Sprintf("msg-%d", i)); err != nil {
				errMu.Lock()
				sendErrors = append(sendErrors, err)
				errMu.Unlock()
			}
		}(i)
	}
	wg.Wait()

	if len(sendErrors) > 0 {
		t.Fatalf("send errors: %v", sendErrors)
	}

	// Wait for all messages to arrive
	waitFor(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(bobGot) >= N
	}, fmt.Sprintf("bob to receive all %d messages", N))

	mu.Lock()
	if len(bobGot) != N {
		t.Fatalf("bob got %d messages, expected %d", len(bobGot), N)
	}
	mu.Unlock()
}

func TestUnreadTracking(t *testing.T) {
	alice, bob, cleanup := setupPair(t)
	defer cleanup()

	done := make(chan struct{})
	bob.OnMessage(func(chatKey string, msg chat.Message) {
		select {
		case done <- struct{}{}:
		default:
		}
	})

	// Send a message
	if err := alice.SendMessage("bob", "unread test"); err != nil {
		t.Fatalf("send: %v", err)
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for message")
	}

	// Bob should have 1 unread from alice
	if n := bob.Unread("alice"); n != 1 {
		t.Fatalf("expected 1 unread, got %d", n)
	}

	// Clear and verify
	bob.ClearUnread("alice")
	if n := bob.Unread("alice"); n != 0 {
		t.Fatalf("expected 0 unread after clear, got %d", n)
	}
}

func TestDeliveryState(t *testing.T) {
	alice, bob, cleanup := setupPair(t)
	defer cleanup()

	done := make(chan struct{})
	bob.OnMessage(func(chatKey string, msg chat.Message) {
		select {
		case done <- struct{}{}:
		default:
		}
	})

	// Alice sends a message
	if err := alice.SendMessage("bob", "delivery test"); err != nil {
		t.Fatalf("send: %v", err)
	}

	// Wait for bob to receive (which triggers ACK)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}

	// Give the ACK time to propagate back
	time.Sleep(100 * time.Millisecond)

	// Alice's message should now be Delivered
	msgs := alice.GetMessages("bob")
	if len(msgs) == 0 {
		t.Fatal("no messages in alice's history")
	}
	if msgs[0].State != chat.StateDelivered {
		t.Fatalf("expected StateDelivered, got %d", msgs[0].State)
	}
}

func TestTypingIndicator(t *testing.T) {
	alice, bob, cleanup := setupPair(t)
	defer cleanup()

	typingReceived := make(chan bool, 1)
	bob.OnTyping(func(chatKey string, isTyping bool) {
		select {
		case typingReceived <- isTyping:
		default:
		}
	})

	// Alice sends typing indicator
	alice.SendTyping("bob", true)

	select {
	case isTyping := <-typingReceived:
		if !isTyping {
			t.Fatal("expected isTyping=true")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for typing indicator")
	}

	// Bob should see alice as typing
	if !bob.IsTyping("alice") {
		t.Fatal("bob should see alice typing")
	}

	// After 3+ seconds, typing should expire
	time.Sleep(3100 * time.Millisecond)
	if bob.IsTyping("alice") {
		t.Fatal("typing should have expired")
	}
}

func TestReaction(t *testing.T) {
	alice, bob, cleanup := setupPair(t)
	defer cleanup()

	msgReceived := make(chan struct{})
	bob.OnMessage(func(chatKey string, msg chat.Message) {
		select {
		case msgReceived <- struct{}{}:
		default:
		}
	})

	reactionReceived := make(chan struct{})
	alice.OnReaction(func(chatKey string, msgID string) {
		select {
		case reactionReceived <- struct{}{}:
		default:
		}
	})

	// Alice sends a message
	if err := alice.SendMessage("bob", "react to this"); err != nil {
		t.Fatalf("send: %v", err)
	}

	select {
	case <-msgReceived:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for message")
	}

	// Get the message ID from bob's perspective
	bobMsgs := bob.GetMessages("alice")
	if len(bobMsgs) == 0 {
		t.Fatal("bob has no messages")
	}
	msgID := bobMsgs[0].ID

	// Bob reacts
	bob.SendReaction("alice", msgID, "\U0001f44d")

	select {
	case <-reactionReceived:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for reaction")
	}

	// Alice should see the reaction on her message
	aliceMsgs := alice.GetMessages("bob")
	found := false
	for _, msg := range aliceMsgs {
		if msg.ID == msgID && len(msg.Reactions) > 0 {
			if msg.Reactions[0].Emoji == "\U0001f44d" {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("alice did not see bob's reaction")
	}
}

func TestStatus(t *testing.T) {
	alice, bob, cleanup := setupPair(t)
	defer cleanup()

	statusReceived := make(chan string, 1)
	bob.OnStatus(func(hostname string, state string) {
		select {
		case statusReceived <- state:
		default:
		}
	})

	// Default status
	if s := alice.GetStatus("alice"); s != "available" {
		t.Fatalf("expected 'available', got %q", s)
	}

	// Alice sets status to busy
	alice.SetStatus("busy")

	select {
	case state := <-statusReceived:
		if state != "busy" {
			t.Fatalf("expected 'busy', got %q", state)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for status update")
	}
}

func TestSearchMessages(t *testing.T) {
	alice, bob, cleanup := setupPair(t)
	defer cleanup()

	done := make(chan struct{})
	bob.OnMessage(func(chatKey string, msg chat.Message) {
		select {
		case done <- struct{}{}:
		default:
		}
	})

	// Send a few messages
	alice.SendMessage("bob", "hello world")
	<-done
	alice.SendMessage("bob", "goodbye world")
	<-done
	alice.SendMessage("bob", "testing 123")
	<-done

	// Search from alice's side (in-memory)
	results := alice.SearchMessages("world")
	total := 0
	for _, msgs := range results {
		total += len(msgs)
	}
	if total != 2 {
		t.Fatalf("expected 2 results for 'world', got %d", total)
	}

	results = alice.SearchMessages("testing")
	total = 0
	for _, msgs := range results {
		total += len(msgs)
	}
	if total != 1 {
		t.Fatalf("expected 1 result for 'testing', got %d", total)
	}
}

func TestReadReceipts(t *testing.T) {
	alice, bob, cleanup := setupPair(t)
	defer cleanup()

	msgReceived := make(chan struct{})
	bob.OnMessage(func(chatKey string, msg chat.Message) {
		select {
		case msgReceived <- struct{}{}:
		default:
		}
	})

	// Alice sends a message
	if err := alice.SendMessage("bob", "read receipt test"); err != nil {
		t.Fatalf("send: %v", err)
	}

	select {
	case <-msgReceived:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}

	// Wait for ACK
	time.Sleep(100 * time.Millisecond)

	// Bob sends read receipts
	bob.SendReadReceipts("alice")

	// Wait for the receipt to propagate
	time.Sleep(200 * time.Millisecond)

	// Alice's message should now be Read
	msgs := alice.GetMessages("bob")
	if len(msgs) == 0 {
		t.Fatal("no messages")
	}
	if msgs[0].State != chat.StateRead {
		t.Fatalf("expected StateRead, got %d", msgs[0].State)
	}
}

func TestReactionEmptyChat(t *testing.T) {
	alice, bob, cleanup := setupPair(t)
	defer cleanup()
	_ = bob

	// SendReaction on a chatKey with no messages should not crash
	alice.SendReaction("bob", "nonexistent-id", "👍")

	// Give async send time to complete
	time.Sleep(50 * time.Millisecond)

	// No messages should exist
	msgs := alice.GetMessages("bob")
	if len(msgs) != 0 {
		t.Fatalf("expected 0 messages, got %d", len(msgs))
	}
}

func TestReactionWrongMessageID(t *testing.T) {
	alice, bob, cleanup := setupPair(t)
	defer cleanup()

	done := make(chan struct{})
	bob.OnMessage(func(chatKey string, msg chat.Message) {
		select {
		case done <- struct{}{}:
		default:
		}
	})

	// Send a message first
	if err := alice.SendMessage("bob", "hello"); err != nil {
		t.Fatalf("send: %v", err)
	}
	<-done

	// React with a wrong message ID — should not crash
	bob.SendReaction("alice", "fake-id-that-does-not-exist", "🔥")

	time.Sleep(50 * time.Millisecond)

	// The message should have no reactions
	msgs := bob.GetMessages("alice")
	if len(msgs) == 0 {
		t.Fatal("expected messages")
	}
	if len(msgs[0].Reactions) != 0 {
		t.Fatalf("expected 0 reactions, got %d", len(msgs[0].Reactions))
	}
}

func TestReactionEmptyEmoji(t *testing.T) {
	alice, bob, cleanup := setupPair(t)
	defer cleanup()

	done := make(chan struct{})
	bob.OnMessage(func(chatKey string, msg chat.Message) {
		select {
		case done <- struct{}{}:
		default:
		}
	})

	if err := alice.SendMessage("bob", "test"); err != nil {
		t.Fatalf("send: %v", err)
	}
	<-done

	// React with empty emoji — should not crash
	bobMsgs := bob.GetMessages("alice")
	bob.SendReaction("alice", bobMsgs[0].ID, "")

	time.Sleep(50 * time.Millisecond)

	// The reaction should be added (even if empty emoji)
	msgs := bob.GetMessages("alice")
	if len(msgs[0].Reactions) != 1 {
		t.Fatalf("expected 1 reaction, got %d", len(msgs[0].Reactions))
	}
}

func TestReactionLocalAndRemote(t *testing.T) {
	// Simulates the exact TUI flow: sender reads messages copy, finds target, reacts
	alice, bob, cleanup := setupPair(t)
	defer cleanup()

	msgReceived := make(chan struct{}, 1)
	bob.OnMessage(func(chatKey string, msg chat.Message) {
		select {
		case msgReceived <- struct{}{}:
		default:
		}
	})

	reactionReceived := make(chan struct{}, 1)
	alice.OnReaction(func(chatKey string, msgID string) {
		select {
		case reactionReceived <- struct{}{}:
		default:
		}
	})

	bobReactionCallback := make(chan struct{}, 1)
	bob.OnReaction(func(chatKey string, msgID string) {
		select {
		case bobReactionCallback <- struct{}{}:
		default:
		}
	})

	// Alice sends a message
	if err := alice.SendMessage("bob", "react to me"); err != nil {
		t.Fatalf("send: %v", err)
	}

	<-msgReceived

	// Simulate TUI flow: bob reads messages copy then reacts
	msgsCopy := bob.GetMessages("alice")
	if len(msgsCopy) == 0 {
		t.Fatal("bob has no messages")
	}

	// Find last non-own, non-system message (like the TUI does)
	var targetID string
	for i := len(msgsCopy) - 1; i >= 0; i-- {
		if !msgsCopy[i].IsOwn && msgsCopy[i].Sender != "system" {
			targetID = msgsCopy[i].ID
			break
		}
	}
	if targetID == "" {
		t.Fatal("no target message found")
	}

	// Bob reacts (this is what the TUI's /react command does)
	bob.SendReaction("alice", targetID, "👍")

	// Verify bob's local reaction was applied immediately
	<-bobReactionCallback
	bobMsgs := bob.GetMessages("alice")
	if len(bobMsgs[0].Reactions) != 1 {
		t.Fatalf("bob should see reaction locally, got %d reactions", len(bobMsgs[0].Reactions))
	}
	if bobMsgs[0].Reactions[0].Emoji != "👍" {
		t.Fatalf("wrong emoji: %q", bobMsgs[0].Reactions[0].Emoji)
	}

	// Wait for alice to receive the reaction
	select {
	case <-reactionReceived:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for reaction on alice's side")
	}

	// Alice should see the reaction
	aliceMsgs := alice.GetMessages("bob")
	found := false
	for _, msg := range aliceMsgs {
		if msg.ID == targetID && len(msg.Reactions) > 0 {
			found = true
			if msg.Reactions[0].Sender != "bob" {
				t.Fatalf("expected sender 'bob', got %q", msg.Reactions[0].Sender)
			}
		}
	}
	if !found {
		t.Fatal("alice did not see bob's reaction")
	}
}

func TestReactionConcurrent(t *testing.T) {
	alice, bob, cleanup := setupPair(t)
	defer cleanup()

	msgReceived := make(chan struct{}, 1)
	bob.OnMessage(func(chatKey string, msg chat.Message) {
		select {
		case msgReceived <- struct{}{}:
		default:
		}
	})

	// Send a message
	if err := alice.SendMessage("bob", "spam react"); err != nil {
		t.Fatalf("send: %v", err)
	}
	<-msgReceived

	bobMsgs := bob.GetMessages("alice")
	msgID := bobMsgs[0].ID

	// Send 10 reactions concurrently from bob
	const N = 10
	emojis := []string{"👍", "👎", "❤️", "🔥", "😂", "😭", "🎉", "🚀", "💯", "✅"}
	var wg sync.WaitGroup
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			bob.SendReaction("alice", msgID, emojis[i])
		}(i)
	}
	wg.Wait()

	// Give async sends time to complete
	time.Sleep(200 * time.Millisecond)

	// Bob should have all N reactions locally
	msgs := bob.GetMessages("alice")
	if len(msgs[0].Reactions) != N {
		t.Fatalf("expected %d reactions, got %d", N, len(msgs[0].Reactions))
	}
}

func TestReactionCallbackFires(t *testing.T) {
	alice, bob, cleanup := setupPair(t)
	defer cleanup()

	msgReceived := make(chan struct{}, 1)
	bob.OnMessage(func(chatKey string, msg chat.Message) {
		select {
		case msgReceived <- struct{}{}:
		default:
		}
	})

	// Track reaction callbacks
	var mu sync.Mutex
	var reactionChatKey, reactionMsgID string
	bob.OnReaction(func(chatKey string, msgID string) {
		mu.Lock()
		reactionChatKey = chatKey
		reactionMsgID = msgID
		mu.Unlock()
	})

	if err := alice.SendMessage("bob", "callback test"); err != nil {
		t.Fatalf("send: %v", err)
	}
	<-msgReceived

	bobMsgs := bob.GetMessages("alice")
	msgID := bobMsgs[0].ID

	bob.SendReaction("alice", msgID, "🎉")

	// Give callback time to fire
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	if reactionChatKey != "alice" {
		t.Fatalf("expected chatKey 'alice', got %q", reactionChatKey)
	}
	if reactionMsgID != msgID {
		t.Fatalf("expected msgID %q, got %q", msgID, reactionMsgID)
	}
	mu.Unlock()
}

func TestGetMessagesDeepCopy(t *testing.T) {
	alice, bob, cleanup := setupPair(t)
	defer cleanup()

	msgReceived := make(chan struct{}, 1)
	bob.OnMessage(func(chatKey string, msg chat.Message) {
		select {
		case msgReceived <- struct{}{}:
		default:
		}
	})

	if err := alice.SendMessage("bob", "deep copy test"); err != nil {
		t.Fatalf("send: %v", err)
	}
	<-msgReceived

	bobMsgs := bob.GetMessages("alice")
	msgID := bobMsgs[0].ID

	// Get a copy of messages BEFORE adding a reaction
	beforeCopy := bob.GetMessages("alice")

	// Add a reaction
	bob.SendReaction("alice", msgID, "👍")
	time.Sleep(50 * time.Millisecond)

	// The "before" copy should NOT have the reaction (deep copy)
	if len(beforeCopy[0].Reactions) != 0 {
		t.Fatalf("deep copy violated: before copy has %d reactions", len(beforeCopy[0].Reactions))
	}

	// A fresh copy SHOULD have the reaction
	afterCopy := bob.GetMessages("alice")
	if len(afterCopy[0].Reactions) != 1 {
		t.Fatalf("expected 1 reaction in fresh copy, got %d", len(afterCopy[0].Reactions))
	}
}

func TestDoubleStop(t *testing.T) {
	alice, _, cleanup := setupPair(t)
	defer cleanup()

	// Calling Stop() twice should not panic
	alice.Stop()
	alice.Stop()
}

// --- File transfer tests ---

func TestSendFileNotFound(t *testing.T) {
	alice, _, cleanup := setupPair(t)
	defer cleanup()

	_, err := alice.SendFile("bob", "/nonexistent/file.txt")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestSendFileDirectory(t *testing.T) {
	alice, _, cleanup := setupPair(t)
	defer cleanup()

	dir := t.TempDir()
	_, err := alice.SendFile("bob", dir)
	if err == nil {
		t.Fatal("expected error for directory")
	}
	if !strings.Contains(err.Error(), "directory") {
		t.Fatalf("expected directory error, got: %v", err)
	}
}

func TestSendFileCreatesPlaceholder(t *testing.T) {
	alice, _, cleanup := setupPair(t)
	defer cleanup()

	// Create a temp file to send
	tmp := t.TempDir()
	path := tmp + "/test.txt"
	os.WriteFile(path, []byte("hello world"), 0644)

	msgID, err := alice.SendFile("bob", path)
	if err != nil {
		t.Fatalf("SendFile: %v", err)
	}
	if msgID == "" {
		t.Fatal("expected non-empty message ID")
	}

	// Verify the placeholder message was created
	msgs := alice.GetMessages("bob")
	found := false
	for _, msg := range msgs {
		if msg.ID == msgID {
			found = true
			if msg.FileInfo == nil {
				t.Fatal("expected FileInfo on placeholder message")
			}
			if msg.FileInfo.Filename != "test.txt" {
				t.Fatalf("expected filename 'test.txt', got %q", msg.FileInfo.Filename)
			}
			if msg.FileInfo.Size != 11 {
				t.Fatalf("expected size 11, got %d", msg.FileInfo.Size)
			}
			if msg.FileInfo.State != chat.FileSending {
				t.Fatalf("expected FileSending state, got %d", msg.FileInfo.State)
			}
			if !msg.IsOwn {
				t.Fatal("expected IsOwn=true")
			}
		}
	}
	if !found {
		t.Fatal("placeholder message not found in history")
	}
}

func TestUpdateFileState(t *testing.T) {
	alice, _, cleanup := setupPair(t)
	defer cleanup()

	// Create and send a file to get a placeholder
	tmp := t.TempDir()
	path := tmp + "/doc.pdf"
	os.WriteFile(path, []byte("pdf content"), 0644)

	msgID, err := alice.SendFile("bob", path)
	if err != nil {
		t.Fatalf("SendFile: %v", err)
	}

	// Update state to FileSent
	alice.UpdateFileState("bob", msgID, chat.FileSent, "")
	msgs := alice.GetMessages("bob")
	for _, msg := range msgs {
		if msg.ID == msgID {
			if msg.FileInfo.State != chat.FileSent {
				t.Fatalf("expected FileSent, got %d", msg.FileInfo.State)
			}
		}
	}

	// Update state to FileFailed with error
	alice.UpdateFileState("bob", msgID, chat.FileFailed, "peer offline")
	msgs = alice.GetMessages("bob")
	for _, msg := range msgs {
		if msg.ID == msgID {
			if msg.FileInfo.State != chat.FileFailed {
				t.Fatalf("expected FileFailed, got %d", msg.FileInfo.State)
			}
			if msg.FileInfo.Error != "peer offline" {
				t.Fatalf("expected error 'peer offline', got %q", msg.FileInfo.Error)
			}
		}
	}
}

func TestUpdateFileStateNonexistent(t *testing.T) {
	alice, _, cleanup := setupPair(t)
	defer cleanup()

	// Should not crash when updating a message that doesn't exist
	alice.UpdateFileState("bob", "nonexistent-id", chat.FileSent, "")
}

func TestFileInfoDeepCopy(t *testing.T) {
	alice, _, cleanup := setupPair(t)
	defer cleanup()

	tmp := t.TempDir()
	path := tmp + "/deep.txt"
	os.WriteFile(path, []byte("deep copy test"), 0644)

	msgID, err := alice.SendFile("bob", path)
	if err != nil {
		t.Fatalf("SendFile: %v", err)
	}

	// Get a copy while FileSending
	before := alice.GetMessages("bob")

	// Update to FileSent
	alice.UpdateFileState("bob", msgID, chat.FileSent, "")

	// The "before" copy should still be FileSending (deep copy)
	for _, msg := range before {
		if msg.ID == msgID {
			if msg.FileInfo.State != chat.FileSending {
				t.Fatalf("deep copy violated: before copy has state %d, expected FileSending", msg.FileInfo.State)
			}
		}
	}

	// A fresh copy should be FileSent
	after := alice.GetMessages("bob")
	for _, msg := range after {
		if msg.ID == msgID {
			if msg.FileInfo.State != chat.FileSent {
				t.Fatalf("expected FileSent in fresh copy, got %d", msg.FileInfo.State)
			}
		}
	}
}

func TestSendFileCallbackFires(t *testing.T) {
	alice, _, cleanup := setupPair(t)
	defer cleanup()

	var mu sync.Mutex
	var callbackChatKey string
	alice.OnMessage(func(chatKey string, msg chat.Message) {
		mu.Lock()
		callbackChatKey = chatKey
		mu.Unlock()
	})

	tmp := t.TempDir()
	path := tmp + "/callback.txt"
	os.WriteFile(path, []byte("callback test"), 0644)

	_, err := alice.SendFile("bob", path)
	if err != nil {
		t.Fatalf("SendFile: %v", err)
	}

	// Give callback time to fire
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	if callbackChatKey != "bob" {
		t.Fatalf("expected callback chatKey 'bob', got %q", callbackChatKey)
	}
	mu.Unlock()
}

// --- Persistence with store tests ---

func setupPairWithStore(t *testing.T) (alice *chat.Manager, bob *chat.Manager, store *storage.Store, cleanup func()) {
	t.Helper()

	kpA, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate alice key: %v", err)
	}
	kpB, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate bob key: %v", err)
	}

	srvA, err := tcnet.NewServer("127.0.0.1:0", kpA, "alice", nil)
	if err != nil {
		t.Fatalf("server alice: %v", err)
	}
	srvA.Start()

	srvB, err := tcnet.NewServer("127.0.0.1:0", kpB, "bob", nil)
	if err != nil {
		srvA.Stop()
		t.Fatalf("server bob: %v", err)
	}
	srvB.Start()

	dir := t.TempDir()
	store, err = storage.New(dir)
	if err != nil {
		srvA.Stop()
		srvB.Stop()
		t.Fatalf("create store: %v", err)
	}

	alice = chat.NewManager(srvA, kpA, "alice", store, nil)
	bob = chat.NewManager(srvB, kpB, "bob", nil, nil)

	conn, err := tcnet.Connect(srvB.Addr(), kpA, "alice", nil)
	if err != nil {
		srvA.Stop()
		srvB.Stop()
		t.Fatalf("connect: %v", err)
	}
	srvA.AddConnection(conn)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if bob.IsConnected("alice") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	cleanup = func() {
		alice.Stop()
		bob.Stop()
		srvA.Stop()
		srvB.Stop()
	}
	return
}

func TestFileInfoPersistenceRoundTrip(t *testing.T) {
	alice, _, store, cleanup := setupPairWithStore(t)
	defer cleanup()

	tmp := t.TempDir()
	path := tmp + "/persist.txt"
	os.WriteFile(path, []byte("persist test"), 0644)

	msgID, err := alice.SendFile("bob", path)
	if err != nil {
		t.Fatalf("SendFile: %v", err)
	}

	// Wait for the initial persist (FileSending) to complete before updating
	time.Sleep(100 * time.Millisecond)

	// Update to FileSent
	alice.UpdateFileState("bob", msgID, chat.FileSent, "")

	// Give async persist time to complete
	time.Sleep(100 * time.Millisecond)

	// Load from disk directly
	loaded, err := store.LoadMessages("bob")
	if err != nil {
		t.Fatalf("LoadMessages: %v", err)
	}

	found := false
	for _, sm := range loaded {
		if sm.ID == msgID {
			found = true
			if sm.FileInfo == nil {
				t.Fatal("FileInfo not persisted")
			}
			if sm.FileInfo.Filename != "persist.txt" {
				t.Fatalf("expected 'persist.txt', got %q", sm.FileInfo.Filename)
			}
			if sm.FileInfo.State != int(chat.FileSent) {
				t.Fatalf("expected state %d (FileSent), got %d", chat.FileSent, sm.FileInfo.State)
			}
		}
	}
	if !found {
		t.Fatal("file message not found in persisted data")
	}
}

func TestGroupChatMessage(t *testing.T) {
	alice, bob, cleanup := setupPair(t)
	defer cleanup()

	group, err := alice.CreateGroup("test-group", []string{"bob"})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}

	bobGotGroup := make(chan struct{}, 1)
	bob.OnMessage(func(chatKey string, msg chat.Message) {
		if strings.HasPrefix(chatKey, "group:") {
			select {
			case bobGotGroup <- struct{}{}:
			default:
			}
		}
	})

	if err := alice.SendGroupMessage(group.ID, "hello group"); err != nil {
		t.Fatalf("send group: %v", err)
	}

	// Verify alice sees her message
	chatKey := "group:" + group.ID
	msgs := alice.GetMessages(chatKey)
	if len(msgs) == 0 {
		t.Fatal("alice has no group messages")
	}
	if msgs[0].Content != "hello group" {
		t.Fatalf("expected 'hello group', got %q", msgs[0].Content)
	}
	if msgs[0].GroupID != group.ID {
		t.Fatalf("expected group ID %q, got %q", group.ID, msgs[0].GroupID)
	}
}

func TestFileTransferE2E(t *testing.T) {
	alice, bob, cleanup := setupPair(t)
	defer cleanup()

	bobGot := make(chan chat.Message, 10)
	bob.OnMessage(func(chatKey string, msg chat.Message) {
		bobGot <- msg
	})

	// Create a test file with known content
	tmp := t.TempDir()
	path := tmp + "/transfer.txt"
	content := "hello, this is a file transfer test!"
	os.WriteFile(path, []byte(content), 0644)

	// Alice sends the file to bob
	msgID, err := alice.SendFile("bob", path)
	if err != nil {
		t.Fatalf("SendFile: %v", err)
	}

	// Wait for bob to receive the file (multiple messages: offer, data, complete callbacks)
	var lastMsg chat.Message
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case msg := <-bobGot:
			lastMsg = msg
		default:
		}

		// Check if bob has a FileReceived message
		msgs := bob.GetMessages("alice")
		for _, msg := range msgs {
			if msg.FileInfo != nil && msg.FileInfo.State == chat.FileReceived {
				// Success! Verify the received file
				if msg.FileInfo.Filename != "transfer.txt" {
					t.Fatalf("expected filename 'transfer.txt', got %q", msg.FileInfo.Filename)
				}
				if msg.FileInfo.Path == "" {
					t.Fatal("expected non-empty path for received file")
				}
				// Read the received file and verify contents
				received, err := os.ReadFile(msg.FileInfo.Path)
				if err != nil {
					t.Fatalf("read received file: %v", err)
				}
				if string(received) != content {
					t.Fatalf("file content mismatch: got %q, want %q", string(received), content)
				}
				// Also check alice's side — should be FileSent
				aliceMsgs := alice.GetMessages("bob")
				for _, am := range aliceMsgs {
					if am.ID == msgID && am.FileInfo != nil {
						if am.FileInfo.State != chat.FileSent {
							t.Fatalf("expected alice's file state FileSent, got %d", am.FileInfo.State)
						}
					}
				}
				return // test passed
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	_ = lastMsg
	t.Fatal("timeout waiting for file transfer to complete")
}

func TestFileTransferNotConnected(t *testing.T) {
	alice, _, cleanup := setupPair(t)
	defer cleanup()

	// Create a file
	tmp := t.TempDir()
	path := tmp + "/offline.txt"
	os.WriteFile(path, []byte("test"), 0644)

	// Try to send to a peer that doesn't exist (no connection)
	_, err := alice.SendFile("nonexistent-peer", path)
	if err != nil {
		t.Fatalf("SendFile should not error at creation time: %v", err)
	}

	// Wait for async send to fail
	time.Sleep(500 * time.Millisecond)

	// The message should be in FileFailed state
	msgs := alice.GetMessages("nonexistent-peer")
	found := false
	for _, msg := range msgs {
		if msg.FileInfo != nil {
			found = true
			if msg.FileInfo.State != chat.FileFailed {
				t.Fatalf("expected FileFailed, got %d", msg.FileInfo.State)
			}
			if msg.FileInfo.Error == "" {
				t.Fatal("expected non-empty error message")
			}
		}
	}
	if !found {
		t.Fatal("file message not found")
	}
}

func TestGroupNonexistent(t *testing.T) {
	alice, _, cleanup := setupPair(t)
	defer cleanup()

	err := alice.SendGroupMessage("nonexistent-group-id", "hello")
	if err == nil {
		t.Fatal("expected error for nonexistent group")
	}
}

func waitFor(t *testing.T, cond func() bool, desc string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %s", desc)
}
