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

	alice = chat.NewManager(srvA, kpA, "alice")
	bob = chat.NewManager(srvB, kpB, "bob")

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
