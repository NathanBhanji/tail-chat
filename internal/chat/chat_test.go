package chat_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/NathanBhanji/tail-chat/internal/chat"
	"github.com/NathanBhanji/tail-chat/internal/crypto"
	tcnet "github.com/NathanBhanji/tail-chat/internal/net"
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

	srvA, err := tcnet.NewServer("127.0.0.1:0", kpA, "alice")
	if err != nil {
		t.Fatalf("server alice: %v", err)
	}
	srvA.Start()

	srvB, err := tcnet.NewServer("127.0.0.1:0", kpB, "bob")
	if err != nil {
		srvA.Stop()
		t.Fatalf("server bob: %v", err)
	}
	srvB.Start()

	alice = chat.NewManager(srvA, kpA, "alice", nil)
	bob = chat.NewManager(srvB, kpB, "bob", nil)

	// Connect alice -> bob
	conn, err := tcnet.Connect(srvB.Addr(), kpA, "alice")
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
